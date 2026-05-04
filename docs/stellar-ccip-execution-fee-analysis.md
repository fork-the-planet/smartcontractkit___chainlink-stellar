# Stellar CCIP Executor: Fee Model, Storage Archival, and Restoration Budget Analysis

## Background

This document captures the investigation and design analysis for the Stellar CCIP executor
(`ccv/contract_transmitter`, `deployment/deployer.go`), focusing on:

1. Deployer reliability issues identified and fixed
2. Soroban storage archival mechanics and the restoration strategy
3. The malicious contract fee-drain attack vector
4. The Soroban gas/budget model and its constraints
5. A proposed design for user-declared restoration budgets and EVM-side fee enforcement

---

## 1. Deployer Reliability Issues (Fixed)

Four reliability gaps were identified in `deployment/deployer.go` and addressed:

### 1.1 Confirmation Timeout < Transaction Time-Bound

**Problem:** The transaction's `MaxTime` (the Stellar network's deadline for accepting it) and
the confirmation poll timeout were computed independently. The original code used
`txnbuild.NewTimeout(300)` for the transaction and a separate 240-second poll window, creating a
window where the poll could expire while the transaction was still valid — or vice versa.

**Fix:** A single `txnDeadline := time.Now().Add(d.txnTimeBound)` is computed once and threaded
through to both the transaction's `MaxTime` (`txnbuild.NewTimebounds(0, txnDeadline.Unix())`) and
the `waitForTransaction` poll. The `txnTimeBound` is configurable via `WithTxnTimeBound(d time.Duration)`,
defaulting to 120 seconds.

### 1.2 FAILED Confirmation Discards Diagnostic XDR

**Problem:** When `GetTransaction` returned `status = "FAILED"`, the error returned no details —
making failures opaque and hard to debug.

**Fix:** The FAILED branch now includes hash, `ResultXDR`, and `DiagnosticEventsXDR` in the
error message:

```
transaction failed (hash: %s, resultXDR: %q, diagnostics: %v)
```

### 1.3 Fixed Fee Buffer

**Problem:** A hardcoded `+100` stroop buffer on top of `MinResourceFee` did not scale with
transaction cost and could leave fees below the network minimum under fee surges.

**Fix:** A percentage-based `feeBumpFactor` (default 1.25 = 25% above minimum) is applied, with a
floor of `minFeeBuffer = 10,000 stroops`. Configurable via `WithFeeBumpFactor(factor float64)`.

### 1.4 Second RestorePreamble Never Checked

**Problem:** After `restoreFootprint` completes and the transaction is re-simulated, the new
`simResult.RestorePreamble` was never checked. If re-simulation still returned a `RestorePreamble`,
the code silently proceeded to submit a transaction that was guaranteed to fail on-chain.

**Fix:** Both `buildAndSubmitTransaction` and `SimulateContract` now return an error if
re-simulation after restore still returns a `RestorePreamble`:

```
simulation after restore still requires another restore: unexpected second RestorePreamble
```

The Stellar RPC captures all archived entries in a single `RestorePreamble` — it does not return
them piecemeal. A second one after a confirmed restore indicates an RPC inconsistency or an entry
that expired in the narrow window between restore confirmation and re-simulation. Erroring allows
the executor to retry from a clean state rather than submitting a guaranteed failure.

---

## 2. Soroban Storage Archival and the Restoration Strategy

### 2.1 Stellar Storage Types

Soroban contracts have three storage types:

| Type | TTL | Expiry behavior |
|------|-----|-----------------|
| Temporary | Own TTL | Permanently deleted — cannot be restored |
| Persistent | Own TTL | Archived — can be restored via `RestoreFootprintOp` |
| Instance | Tied to contract instance TTL | Archived with contract instance — restorable |

Only Persistent and Instance storage can be restored. Temporary storage is gone forever after expiry.

### 2.2 Restoration Flow (Pre-Protocol 23)

When a `SimulateTransaction` response includes a `RestorePreamble`, it means the transaction's
footprint references one or more archived entries. The required flow is:

```
SimulateTransaction
  └─ RestorePreamble present?
       ├─ Yes → submit RestoreFootprintOp (separate transaction)
       │         └─ wait for confirmation
       │         └─ re-simulate original transaction
       │              └─ submit original transaction
       └─ No  → submit original transaction directly
```

The `RestorePreamble` contains:
- `TransactionDataXDR`: the full footprint of all archived entries to restore
- `MinResourceFee`: the minimum fee required for the restore transaction

The Stellar RPC returns **all** archived entries in a single `RestorePreamble`. It is not
piecemeal; if re-simulation after a confirmed restore still returns a `RestorePreamble`, that is
an unexpected condition (see §1.4).

### 2.3 Protocol 23 Auto-Restoration (CAP-0066)

From Protocol 23 onward, `InvokeHostFunctionOp` automatically restores archived entries that
appear in its footprint via `SorobanResourcesExtV0.archivedSorobanEntries`. The manual
`RestoreFootprintOp` is no longer needed.

Key details:
- **TTL after auto-restore:** Entries are set to `minPersistentEntryTTL` (~4,096 ledgers ≈ 5.7 hours
  at 5-second ledger close). This is the minimum — not a long extension.
- **Who pays:** The transaction submitter (the executor) pays the restoration fee automatically as
  part of the `InvokeHostFunctionOp` fee. There is no opt-out.
- **Applies to all contracts:** Auto-restore applies regardless of when the contract was deployed
  or which Soroban SDK version it uses.
- **Current deployer behavior:** The existing `RestoreFootprintOp` path is harmless redundant
  overhead on Protocol 23+ networks. `WithAutoRestore(false)` can disable it for operators who
  know they are on Protocol 23+.

---

## 3. Soroban Budget / Gas Model

Soroban has a resource-based fee model, distinct from EVM gas:

| Resource | Type | Behavior |
|----------|------|----------|
| `instructions` | CPU | Hard cap; transaction fails if exceeded. Fee charged on declared value (non-refundable). |
| `readBytes` | I/O | Hard cap. Fee charged on declared value (non-refundable). |
| `writeBytes` | I/O | Hard cap. Fee charged on declared value (non-refundable). |
| Rent/events | Storage | Charged upfront; refunded based on actual usage (refundable). |

**Critical distinction from EVM:** Non-refundable resources (instructions, bytes) are charged
based on the **declared** limit from simulation, not actual usage. Declaring less than needed
causes an on-chain failure but the declared fee is still consumed.

### 3.1 `gas_limit_override` in the Stellar OffRamp

The Rust OffRamp's `execute` function accepts `_gas_limit_override: u32` (note the underscore
prefix — intentionally unused in execution). It is only validated (must be ≥ the message's
`ccip_receive_gas_limit`) but is never applied to the Soroban transaction's instruction budget.

**Consequence:** The `gasLimit` field set by EVM senders currently has no effect on the actual
Soroban execution cost or the restoration fee. All resource limits come from simulation output.

### 3.2 Restoration Fees Are Separate from Execution Instructions

Storage restoration fees (driven by footprint size and TTL duration) are entirely separate from
the execution instruction budget. There is no mechanism to "cap" restoration cost — either all
archived entries in the footprint are restored (at `MinResourceFee`) or the transaction cannot
proceed. The only control is to **refuse to restore** if the cost exceeds a threshold.

---

## 4. Malicious Contract Fee-Drain Attack

**Attack vector:** A malicious EVM→Stellar cross-chain message targets a contract on Stellar that
has been pre-loaded with a large number of expired persistent storage entries. When the Stellar
executor simulates the call, the `RestorePreamble` includes all these entries, and
`MinResourceFee` is very high. The executor pays this restoration fee from its own XLM wallet
before it can even attempt to execute the message.

**Why this is a real threat:**
- The Stellar executor pays for restoration unconditionally (no on-chain cap mechanism exists)
- The EVM fee the user paid does not automatically cover XLM restoration costs
- A single carefully crafted contract could drain the executor's XLM balance across many messages

**Mitigation:** The executor must enforce a per-message restoration budget cap before calling
`restoreFootprint`. If `RestorePreamble.MinResourceFee` exceeds the cap, the executor returns an
error and the message is not executed. The cap value should be communicated from the EVM sender
to the Stellar executor via the message's `executorArgs` (see §5).

---

## 5. Proposed Design: StellarExecutorArgsV1 and Restoration Budget

### 5.1 Cross-Chain Communication of Restoration Budget

Following the existing pattern in `ExtraArgsCodec.sol` (see `SVMExecutorArgsV1` for Solana,
`SuiExecutorArgsV1` for Sui), a new `StellarExecutorArgsV1` struct should be added to
`ExtraArgsCodec.sol` in `chainlink-ccip`:

```solidity
struct StellarExecutorArgsV1 {
    // Maximum stroops the executor may spend on storage restoration for this message.
    // 0 means "use the executor's configured default cap."
    uint64 maxRestorationFeeStoops;
}
```

The EVM sender encodes this into `executorArgs` of `GenericExtraArgsV3`.

### 5.2 How it Flows Through the OnRamp (No OnRamp Changes Needed)

`OnRamp.forwardFromRouter` already passes `executorArgs` verbatim into `MessageV1.destBlob`
(`destBlob: resolvedExtraArgs.executorArgs`). This field is part of the signed message ID — so
`maxRestorationFeeStoops` is tamper-evident end-to-end.

On the Stellar side, the executor decodes `destBlob` → `StellarExecutorArgsV1`, extracts
`maxRestorationFeeStoops`, and passes it to the deployer as a pre-restore budget check:

```
if RestorePreamble.MinResourceFee > maxRestorationFeeStoops → return error (don't restore)
else → proceed with restoreFootprint
```

### 5.3 The Remaining Gap: EVM-Side Pre-Payment

`StellarExecutorArgsV1` lets the user specify a cap and protects the executor from overspending,
but the EVM fee the user pays does not automatically cover the XLM the executor will spend on
restoration. Two solutions are proposed:

---

#### Solution A: Executor EVM `getFee` Charges for `maxRestorationFeeStoops`

The Stellar executor's EVM-side contract implements `IExecutor.getFee`. This function already
receives `executorArgs`. It decodes `StellarExecutorArgsV1` and adds the restoration cost to
its quoted fee:

```
executor_fee = base_flat_fee + (maxRestorationFeeStoops × XLM_USD_price / feeToken_USD_price)
```

The XLM price is sourced from the FeeQuoter (XLM registered as a priced asset). The OnRamp
collects this fee and transfers it to the executor's EVM contract. The operator uses this pool
to replenish the Stellar keypair's XLM off-chain (or via a treasury bridge).

**Pros:**
- User pays exactly what they declared they would spend on restoration
- Executor is fully reimbursed per message
- `maxRestorationFeeStoops` serves dual purpose: budget cap on Stellar and billable amount on EVM
- No free-rider problem — every user pays for their own restoration cost

**Cons:**
- Requires XLM registered as a priced asset in the FeeQuoter
- Requires the Stellar executor's EVM contract to be updated to decode `StellarExecutorArgsV1`
- If the user sets `maxRestorationFeeStoops` too low, the message fails to execute on Stellar
  even though they paid (though they can retry with a higher value)

**Implementation steps:**

*In `chainlink-ccip` (Solidity):*

1. **`chains/evm/contracts/libraries/ExtraArgsCodec.sol`** — Add `StellarExecutorArgsV1`
   following the `SVMExecutorArgsV1`/`SuiExecutorArgsV1` pattern:
   - New tag constant: `bytes4 public constant STELLAR_EXECUTOR_ARGS_V1_TAG = 0x...;`
   - New struct: `struct StellarExecutorArgsV1 { uint64 maxRestorationFeeStoops; }`
   - `_encodeStellarExecutorArgsV1(StellarExecutorArgsV1 memory) internal pure returns (bytes memory)`
   - `_decodeStellarExecutorArgsV1(bytes calldata) internal pure returns (StellarExecutorArgsV1 memory)`

2. **Stellar executor EVM contract** (whichever contract implements `IExecutor` for Stellar) —
   Update `getFee(destChainSelector, requestedFinalityConfig, ccvs, executorArgs, feeToken)`:
   - Check if `executorArgs` starts with `STELLAR_EXECUTOR_ARGS_V1_TAG`; if so, decode it
   - If `maxRestorationFeeStoops > 0`, convert to fee token via FeeQuoter:
     `restorationFee = IFeeQuoter.convertTokenAmount(XLM_address, maxRestorationFeeStoops, feeToken)`
   - Return `base_flat_fee + restorationFee`

3. **FeeQuoter configuration** (operational, not a code change) — Register XLM with a price
   feed so `convertTokenAmount` can price stroops in any fee token.

*In `chainlink-stellar` (Go):*

4. **`ccv/contract_transmitter/stellar_executor_args.go`** (new file) — Decoder for
   `StellarExecutorArgsV1` from raw bytes:
   ```go
   // Wire format: 4-byte tag + 8-byte big-endian uint64
   func decodeStellarExecutorArgsV1(b []byte) (maxRestorationFeeStoops uint64, err error)
   ```

5. **`deployment/deployer.go`** — Add a context-based per-invocation restoration budget:
   ```go
   type ctxKeyRestorationBudget struct{}

   // WithRestorationBudget attaches a per-message cap to the context.
   func WithRestorationBudget(ctx context.Context, maxStoops int64) context.Context

   // In buildAndSubmitTransaction, before calling restoreFootprint:
   if budget, ok := ctx.Value(ctxKeyRestorationBudget{}).(int64); ok && budget > 0 {
       if simResult.RestorePreamble.MinResourceFee > budget {
           return nil, fmt.Errorf("restoration fee %d exceeds message budget %d stroops", ...)
       }
   }
   ```
   Using context (rather than a Deployer field) keeps the check stateless and safe for
   concurrent calls through the same `Deployer` instance.

6. **`ccv/contract_transmitter/contract_transmitter.go`** —
   `ConvertAndWriteMessageToChain` already has access to `report.Message.DestBlob` (the field
   that carries `executorArgs` end-to-end). Add:
   ```go
   args, err := decodeStellarExecutorArgsV1(report.Message.DestBlob)
   if err == nil && args.maxRestorationFeeStoops > 0 {
       ctx = deployment.WithRestorationBudget(ctx, int64(args.maxRestorationFeeStoops))
   }
   // existing: ct.offrampClient.Execute(ctx, ...)
   ```

---

#### Solution B: Flat Restoration Buffer in the Executor's Base Fee

No per-message restoration fee. The executor operator estimates a worst-case restoration cost
(e.g., 0.2 XLM per message), converts it to fee token at configuration time, and bakes it into
the executor's flat fee returned by `IExecutor.getFee`. Every Stellar message pays this buffer
regardless of whether restoration actually occurs.

The executor enforces a hard cap on Stellar side configured by the operator. If restoration
would exceed the cap, the message fails rather than draining the executor's wallet.

**Pros:**
- Zero new infrastructure — no XLM price oracle needed
- Works with the existing executor fee structure today
- Simpler to reason about and operate

**Cons:**
- All users pay the buffer even when no restoration is needed (token-only transfers, contracts
  with healthy TTLs)
- A malicious contract that exactly hits the buffer costs the executor nothing extra but still
  forces a restore transaction; the buffer must be sized conservatively
- No per-user accountability — high-restoration users subsidized by low-restoration users

**Implementation steps:**

*In `chainlink-stellar` (Go):*

1. **`ccv/contract_transmitter/contract_transmitter.go`** (`ContractTransmitterConfig`) — Add
   one field:
   ```go
   // MaxRestorationFeeStoops is the maximum stroops the executor will spend on a
   // RestoreFootprintOp for any single message. 0 disables the cap (unsafe for production).
   MaxRestorationFeeStoops int64 `toml:"max_restoration_fee_stoops"`
   ```

2. **`deployment/deployer.go`** — Add option and field:
   ```go
   // In Deployer struct:
   maxRestorationFee int64

   // New option:
   func WithMaxRestorationFee(maxStoops int64) DeployerOption {
       return func(d *Deployer) { d.maxRestorationFee = maxStoops }
   }

   // In buildAndSubmitTransaction, before restoreFootprint:
   if d.maxRestorationFee > 0 && simResult.RestorePreamble.MinResourceFee > d.maxRestorationFee {
       return nil, fmt.Errorf("restoration fee %d stroops exceeds configured cap %d stroops",
           simResult.RestorePreamble.MinResourceFee, d.maxRestorationFee)
   }
   ```

3. **`ccv/executorbootstrap/stellar.go`** — Pass the cap when constructing the `Deployer`:
   ```go
   invoker := deployment.NewDeployer(
       rpcClient,
       tc.NetworkPassphrase,
       deployerKeypair,
       deployment.WithMaxRestorationFee(tc.MaxRestorationFeeStoops),
   )
   ```

*In `chainlink-ccip` (Solidity):*

4. **No code change required.** The executor operator increases the executor EVM contract's
   configured flat fee to include the restoration buffer estimate (via whatever admin setter the
   executor contract exposes). No structural Solidity changes are needed for Solution B.

---

#### Recommendation

Ship Solution B immediately to establish safety (executor never spends more than the configured
cap, cost baked into flat fee). Migrate to Solution A once an XLM price feed is available in the
FeeQuoter, enabling precise per-message billing.

---

## 6. Summary of Open Work Items

| Item | Status | Notes |
|------|--------|-------|
| Confirmation timeout / time-bound alignment | Fixed | `WithTxnTimeBound`, shared deadline |
| FAILED confirmation diagnostic surfacing | Fixed | Hash + ResultXDR + DiagnosticEventsXDR in error |
| Percentage-based fee bump | Fixed | `WithFeeBumpFactor`, `minFeeBuffer` floor |
| Second RestorePreamble check | Fixed | Error returned after re-simulation |
| `gas_limit_override` unused in OffRamp | Known gap | Future work: wire to Soroban instruction cap |
| Malicious contract restoration drain | Unmitigated | `StellarExecutorArgsV1.maxRestorationFeeStoops` cap (§5) |
| EVM user pre-payment for restoration | Unmitigated | Solution A (precise) or Solution B (buffered) (§5.3) |
| Deterministic keypair in executorbootstrap | Security gap | TODO exists; replace with env-injected key |
