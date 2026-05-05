# MCMS-Stellar specification (v1 draft)

Normative references:

- [`ManyChainMultiSig.sol`](https://github.com/smartcontractkit/ccip-owner-contracts/blob/main/src/ManyChainMultiSig.sol) — behavior and invariants to mirror on Soroban.
- [`mcms` proposal / Merkle / signing](https://github.com/smartcontractkit/mcms) — off-chain proposal JSON, leaf ordering, and `SigningMessage` construction.

This document pins **hash preimages**, **chain binding**, **`set_root` signing**, and **JSON field conventions** so Soroban, Go (`mcms/sdk/stellar`), and existing signer tooling stay aligned.

---

## 1. Goals

1. **Merkle leaves** computed off-chain (`mcms` encoder) **must equal** leaves verified on-chain (byte-for-byte `keccak256` inputs).
2. **`set_root` ECDSA** must match Solidity: same **message hash** and **EIP-191** personal-sign envelope as `ManyChainMultiSig.setRoot`.
3. **Signer identities** are **Ethereum-style addresses** (20-byte secp256k1 **uncompressed** pubkey → keccak → last 20 bytes), same as EVM MCMS — so existing **Ledger / `mcms` signers** keep working.
4. **Network binding** uses the **Stellar network passphrase hash** published in [`chain-selectors`](https://github.com/smartcontractkit/chain-selectors) (`StellarChainIdFromSelector`), not a runtime ledger value.

### 1.1 Layering (same mental model as Sui MCMS)

This spec intentionally mirrors **Sui MCMS** ([`mcms.move`](https://github.com/smartcontractkit/chainlink-sui/blob/develop/contracts/mcms/mcms/sources/mcms.move)):

| Layer | Sui | Stellar (this spec) |
|-------|-----|------------------------|
| **Merkle / leaf hash** | `keccak256` over a fixed packing (domain separator + typed fields + **payload bytes**) | **`keccak256`** over **`abi.encode(D_*, StellarOp)`** / metadata tuple (§5) — Ethereum-style packing for **non-payload** fields keeps Go/Rust tooling straightforward. |
| **`data` inside the op** | **`vector<u8>`** filled with **Move-native** serialized arguments (**BCS** per field values; `op.data` is hashed as `bcs::to_bytes(&op.data)` in the leaf) | **`bytes data`** filled with **Stellar-native** serialized invoke arguments (see §8): whatever the Soroban host / SDK uses to represent **symbol + `ScVal` args** for the target contract. **Not** EVM ABI calldata. |

**Why this is the path of least resistance**

- **Mental model:** Anyone who understands **Sui MCMS** already knows: **one recipe for the leaf hash**, **separate story for “what goes in `data`”.** Stellar follows the same split; only the native payload format changes (BCS args on Sui → Soroban **`ScVal` / SDK bytes** on Stellar).
- **Off-chain tooling:** The **`mcms` encoder** only needs a **deterministic byte string** for `StellarOp.data` (same bytes in proposal JSON `transaction.data`, same bytes on-chain). Builders produce those bytes with **Stellar SDKs** (`soroban-client`, JS/Rust CLI, etc.), not with Solidity ABI codecs. The **leaf hash** stays stable **Keccak over the outer struct**, including **`data` as opaque bytes** — exactly how opaque calldata sits inside EVM `abi.encode(Op)` and opaque BCS sits inside Sui’s leaf packing.

---

## 2. Cryptographic primitives

| Name | Definition |
|------|------------|
| `keccak256` | Keccak-256 as in Ethereum / `tiny-keccak` (same as Soroban SDK crypto). |
| **Merkle pair hash** | Same as `mcms` / OpenZeppelin: `H(a,b) = keccak256(concat(sort(a,b)))` where `sort` orders two `bytes32` lexicographically (see [`mcms/internal/core/merkle`](https://github.com/smartcontractkit/mcms)). Odd leaf duplication at each level matches current `mcms` tree builder. |
| **Leaf ordering in tree** | After hashing all metadata and op leaves to `bytes32`, **sort leaves by hex string** (same as [`Proposal.MerkleTree`](https://github.com/smartcontractkit/mcms/blob/main/proposal.go)). |

---

## 3. Domain separators (distinct from EVM / Sui)

UTF-8 strings; **constant hash** is `keccak256` of the string (32-byte word).

```
MANY_CHAIN_MULTI_SIG_DOMAIN_SEPARATOR_OP_STELLAR
MANY_CHAIN_MULTI_SIG_DOMAIN_SEPARATOR_METADATA_STELLAR
```

Implementations **must not** reuse `MANY_CHAIN_MULTI_SIG_DOMAIN_SEPARATOR_OP` / `_METADATA` without `_STELLAR` — cross-family leaf collisions in multi-chain trees must be impossible.

---

## 4. Chain binding: `chain_id_u256`

**Source:** `chain-selectors` Stellar entry for the deployment’s [`ChainSelector`](https://github.com/smartcontractkit/chain-selectors): field **`ChainID`** is a **64-character hex string** (no `0x`), 32 bytes — the **network passphrase hash**.

**Encoding for ABI:**

- Decode hex → `bytes32 network_id`.
- **`chain_id_u256`**: interpret `network_id` as **unsigned big-endian uint256** (same integer convention as passing a `bytes32` network id through Solidity’s `uint256` for hashing).

This value appears in **both** `StellarOp` and `StellarRootMetadata` preimages (analogous to `block.chainid` / `chainId` on EVM).

On-chain, `execute` **must** reject ops whose `chain_id_u256` does not match the contract’s configured network binding (set at deploy or fixed in Wasm for a given build).

---

## 5. Merkle leaf types (ABI-encoded preimages)

Leaf hashes are:

```
leaf_op       = keccak256(abi.encode(D_OP_STELLAR, stellarOp))
leaf_metadata = keccak256(abi.encode(D_META_STELLAR, stellarRootMetadata))
```

where `D_OP_STELLAR` and `D_META_STELLAR` are the **32-byte constant words** from §3, and `abi.encode` follows **Solidity ABI v1** packing (32-byte word alignment, `bytes` length + data).

Solidity-equivalent structs:

```solidity
// Hashed as the second argument to abi.encode alongside the domain word.
struct StellarRootMetadata {
    uint256 chainId;               // §4 chain_id_u256
    bytes32 multiSig;              // this MCMS contract id (32 bytes)
    uint40 preOpCount;
    uint40 postOpCount;
    bool overridePreviousRoot;
}

struct StellarOp {
    uint256 chainId;               // must match §4 for this deployment
    bytes32 multiSig;              // must equal this MCMS contract id (32 bytes)
    uint40 nonce;                  // op index for this MCMS instance; executes in order
    bytes32 to;                    // target Soroban contract id (32 bytes)
    uint256 value;                 // reserved; MUST be 0 in v1 (no native XLM attach)
    bytes data;                    // opaque Soroban invocation payload (see §8)
}
```

**Note:** EVM uses `address` (20 bytes) for `multiSig` and `to`; Stellar uses **`bytes32`** contract ids everywhere in these structs. Go and Soroban must implement the **same** ABI tuple types.

**Merkle safety:** Preimages **must** be long enough that leaves do not collide with internal Merkle nodes (same rationale as Solidity comments on `ManyChainMultiSig`). The above tuples with non-trivial `bytes data` satisfy this for typical ops.

---

## 6. `set_root` signatures (EVM parity)

### 6.1 Message digest

Matches [`ManyChainMultiSig.setRoot`](https://github.com/smartcontractkit/ccip-owner-contracts/blob/main/src/ManyChainMultiSig.sol) and [`mcms` `Proposal.SigningMessage`](https://github.com/smartcontractkit/mcms/blob/main/proposal.go):

1. `inner = keccak256(abi.encode(bytes32 root, uint32 validUntil))`  
   — ABI types **`bytes32`,`uint32`** (same as `mcms` `SignMsgABI`).
2. `signedHash = EIP191(inner)` where EIP-191 is the Ethereum **personal_sign** scheme used by `ECDSA.toEthSignedMessageHash` on Solidity **for a 32-byte inner hash**:

   `keccak256(concat(0x19, "Ethereum Signed Message:\n32", inner))`

   (Prefix **decimal** length `32`, not ASCII `"32"`—standard OpenZeppelin `MessageHashUtils.toEthSignedMessageHash(bytes32)`.)

On `set_root`, the contract records replay protection on **`signedHash`** (same idea as `s_seenSignedHashes` on EVM).

### 6.2 Signatures

- **Curve:** secp256k1.
- **Encoding:** `(v, r, s)` with **low-s** enforcement recommended (match OZ). Signatures **sorted by recovered Ethereum address strictly increasing** (same as Solidity).
- **Signer set:** Config stores **Ethereum address** per signer for recovery matching (§1.3).

### 6.3 `validUntil`

- Type **`uint32`** everywhere: proposal JSON, MCMS storage, and ABI for `inner` hash.
- Semantics: **Unix timestamp (seconds)** after which the root **must not** be used — aligned with EVM `ManyChainMultiSig` (`block.timestamp` comparison). **Not** Soroban ledger sequence.
- **Maximum horizon:** on `set_root`, `validUntil` must be ≤ **current ledger timestamp + effective_max_secs**, where:

  ```
  effective_max_secs = min(
      MAX_ROOT_VALIDITY_SECS,                                                      // static absolute cap (90 days)
      LEDGER_BUMP * min_secs_per_ledger - SEEN_TTL_SAFETY_MARGIN_SECS               // dynamic cap
  )
  ```

  The dynamic cap binds `validUntil` to the worst-case lifetime of a freshly bumped `SeenHash` entry minus a 1-week safety margin, so the entry is **guaranteed** to outlive `validUntil` and replay protection cannot lapse via TTL archival. `min_secs_per_ledger` defaults to **5** (`MIN_SECS_PER_LEDGER_DEFAULT`) and is owner-configurable via `set_min_secs_per_ledger(secs)` (gated like `set_config`); read the current value with **`get_min_secs_per_ledger`**. Valid range is `[MIN_SECS_PER_LEDGER_LOWER_BOUND, MIN_SECS_PER_LEDGER_UPPER_BOUND]`. A compile-time assertion in `constants.rs` enforces that the static cap stays strictly below the default-pessimistic seen-entry lifetime so the relation cannot silently regress.

### 6.4 `SeenHash` TTL & restoring archived entries

Replay-protection entries use the **signed hash `h` itself** (`BytesN<32>`) as the persistent storage key. They are created (and bumped to `LEDGER_BUMP`) once at successful `set_root`, and are intentionally **not** refreshed by `extend_all_ttls` / `bump_ttls`: they are not enumerable from inside the contract, and the §6.3 dynamic `validUntil` cap already guarantees that any `(root, validUntil)` whose seen-hash entry could be archived has `validUntil < now` and is therefore rejected before the seen-hash check runs. **Replay safety does not require archived seen entries to be readable.**

There is **no** guest-side "restore seen hash" entrypoint on the MCMS contract, and one cannot exist:

- Soroban does **not** expose a `restore` host function to guest contracts; the only TTL primitive available to a contract is `extend_ttl`, which requires the entry to already be live.
- Restoration is a **host-level operation** (`RestoreFootprintOp`) submitted as part of a Stellar transaction. The transaction submitter declares the archived ledger keys in the operation's footprint, the host re-instates them, and rent is paid by the submitter. The contract has no role in this flow.

If you ever need to read an archived `SeenHash` entry (e.g. for governance forensics or audit), the standard procedure is:

1. Compute the ledger key for persistent contract data keyed by `h` (`BytesN<32>`) against the MCMS contract id.
2. Submit a transaction that includes a `RestoreFootprintOp` listing that key in its read-write footprint.
3. After the host pays rent and re-instates the entry, read it via `getLedgerEntries` RPC or a contract-side getter.

This mirrors the pattern documented for `timelock` per-op timestamps (`contracts/timelock/src/lib.rs`) and is consistent with idiomatic Soroban: contracts manage TTL of **live** entries via `extend_ttl`; restoration of archived entries is the **submitter's** responsibility, not the contract's.

---

## 7. Signer configuration (logical `Config`)

Mirror Solidity `Config` shape for **authorization / quorum logic** portability:

- **`NUM_GROUPS`** = 32, **`MAX_NUM_SIGNERS`** = 200.
- **`Signer`**: `{ address addr; uint8 index; uint8 group }` — `addr` is **20-byte Ethereum address** (ABI `address`), **not** a Stellar contract address.
- **`groupQuorums`**, **`groupParents`**: `uint8[32]` with the same tree constraints as Solidity (`groupParents[0]==0`, parent index `<` child index, etc.).

On-chain Soroban may store addresses as `BytesN<20>` or `(u128,u32)` packs; verification compares bytes to **`ecrecover`** output.

---

## 8. Soroban invocation payload (`StellarOp.data`)

**Normative:** `StellarOp.data` **must** carry **Stellar-native** serialization of the arguments MCMS will pass through to **`invoke_contract`** (contract `to`, function symbol, **`ScVal` arguments**), i.e. bytes produced by **canonical Soroban / Stellar SDK** conventions — **not** EVM ABI, and **not** Sui BCS.

Concretely:

- **Proposal authors** build `transaction.data` using the same rules the **executor** and **MCMS contract** use to unpack and invoke (golden vectors must match).
- The **Merkle leaf** still hashes `data` as **opaque `bytes`** inside **`abi.encode`** (§5): the leaf uses **Keccak256** at the outer layer; the **contents** of `data` are **native Stellar** payloads (analogous to Sui: leaf uses Keccak over a packing that includes **`bcs::to_bytes(&op.data)`** for the payload slot).

Exact **per-type `ScVal`** layout (tuple order, symbol encoding, etc.) is **pinned in the Soroban MCMS implementation** and mirrored in **`mcms/sdk/stellar`** helper functions — this spec only requires **native encoding for `data`** and **byte-identical** hashing on-chain and in Go.

**Restriction:** **`value`** field **must be 0** in v1; native XLM attachment is out of scope.

---

## 9. `mcms` proposal JSON ↔ Stellar (`types.Operation`)

Uses [`types.ChainMetadata`](https://github.com/smartcontractkit/mcms/blob/main/types/chain.go) and [`types.Operation`](https://github.com/smartcontractkit/mcms/blob/main/types/operation.go).

### 9.1 `chainMetadata` for Stellar chains

| Field | Stellar convention |
|-------|---------------------|
| `mcmAddress` | MCMS contract id: **StrKey `C…`** Stellar contract address string (canonical form). Encoder **normalizes** to **32-byte raw contract id** for hashes. |
| `startingOpCount` | Matches `preOpCount` / `StellarRootMetadata.preOpCount` for the proposal batch. |
| `additionalFields` | See §9.3 |

### 9.2 `transaction` fields

| Field | Stellar convention |
|-------|---------------------|
| `to` | **Target** governed contract StrKey (`C…`) for the Soroban **invoke**. Must match `StellarOp.to` bytes32 in the leaf hash. |
| `data` | **Stellar-native** serialized invoke args for `to` (see §8); same bytes as `StellarOp.data` in the leaf preimage. Typically produced by Soroban/Stellar SDKs, **not** ABI/BCS-from-other-chains. |
| `additionalFields` | See §9.3 |

### 9.3 `additionalFields` schema (versioned)

Minimal v1 envelope (both metadata and tx level where applicable):

```json
{
  "version": 1,
  "family": "stellar"
}
```

Optional keys (reserved for Phase 6 executor / timelock wiring):

| Key | Purpose |
|-----|---------|
| `timelock_action` | `"none"` \| `"schedule_batch"` \| `"cancel"` \| `"bypass_execute_batch"` — when embedded timelock exists (future). |
| `notes` | Non-normative string for operators. |

**Rule:** Unknown JSON keys **should** be rejected by strict encoders or ignored only if documented; **version** bumps when semantics change.

---

## 10. Soroban `Address` and authorization

- External **invoke** from MCMS runs as **MCMS contract** as invoker; **governed contracts** must recognize **owner / admin == MCMS contract address** where applicable.
- **`require_auth`**: Callees **must** accept auth from the MCMS contract address for governed operations; MCMS implementation **must** call targets with correct Soroban auth semantics for its environment (document in implementation).

---

## 11. ABI fragments for tooling (informative)

Encoder authors can validate against:

**Metadata tuple** (second arg to `abi.encode` with domain word):

```
(uint256,bytes32,uint40,uint40,bool)
```

**Op tuple**:

```
(uint256,bytes32,uint40,bytes32,uint256,bytes)
```

**Signing inner hash**:

```
(bytes32,uint32)
```

---

## 12. Future work (out of scope for this spec file)

- Embeddable **RBACTimelock**-equivalent **blocked selector** definition on Soroban (first N bytes of `data` vs symbol hash).
- **`mcms/sdk/stellar`** Go package and `FamilyStellar` registration in `mcms`.

---

## Revision history

| Version | Date | Notes |
|---------|------|------|
| v1 draft | — | Phase 1 spec; merged governance plan; §1.1/§8 Sui-like layering (`keccak256` leaves + Stellar-native `data`); StrKey-only in JSON (§9). |
