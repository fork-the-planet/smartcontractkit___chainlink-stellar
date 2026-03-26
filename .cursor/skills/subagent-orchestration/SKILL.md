---
name: subagent-orchestration
description: "Orchestration guide for invoking specialized sub-agents during coding workflows. Defines when and how to invoke testing-engineer, security-auditor, quality-assurance-reviewer, context-keeper, and project-tracker agents. Use proactively after implementing features, modifying security-sensitive code, completing milestones, or when the user asks for reviews, tests, or status updates."
---

# Sub-Agent Orchestration

This skill tells you **when** to invoke specialized sub-agents and **how** to combine them effectively. The goal is to make sub-agent usage a natural part of the workflow, not an afterthought.

## Available Sub-Agents

| Agent | Type | Mode | Purpose |
|-------|------|------|---------|
| `testing-engineer` | Background | Read/Write | Write tests for new or modified code |
| `security-auditor` | Background | Readonly | Vulnerability assessment on security-sensitive changes |
| `quality-assurance-reviewer` | Background | Readonly | Code quality, readability, and maintainability review |
| `context-keeper` | Background | Read/Write | Persist confirmed knowledge to the graph |
| `project-tracker` | Background | Read-only | Cross-reference Jira, Git, and knowledge graph for status |

## Trigger Rules

Follow these rules to decide when to invoke sub-agents. Apply them **proactively** — don't wait for the user to ask.

### After Implementing or Modifying a Feature

**Trigger:** You've written or substantially changed functional code (not just config, docs, or formatting).

**Action:** Invoke up to three agents in parallel:

```
┌─────────────────────┐
│  Feature Complete    │
└──────┬──────────────┘
       │ parallel
       ├──→ testing-engineer      (write tests for the changes)
       ├──→ quality-assurance-reviewer  (review code quality)
       └──→ context-keeper        (persist architectural decisions)
```

**Prompt patterns:**

- **testing-engineer:** "Write tests for the changes in `[files]`. The feature does [description]. Focus on [core behavior, error handling, edge cases]. Follow existing test conventions in `[test directory]`."
- **quality-assurance-reviewer:** "Review the code in `[files]` for quality and maintainability. The changes implement [description]."
- **context-keeper:** "Persist the following confirmed architectural decision: [decision and rationale]. Group ID: [appropriate group]."

### When Touching Security-Sensitive Code

**Trigger:** Changes involve any of:
- Token transfers, minting, burning, locking
- Cross-chain message passing or verification
- Access control, permissions, admin functions
- Cryptographic operations (signing, hashing, proofs)
- External contract calls
- State transitions with financial impact

**Action:** Invoke `security-auditor` alongside any other applicable agents:

```
┌────────────────────────┐
│  Security-Sensitive     │
│  Code Changed           │
└──────┬─────────────────┘
       │ parallel
       ├──→ security-auditor      (vulnerability assessment)
       ├──→ testing-engineer      (security-focused test cases)
       └──→ context-keeper        (persist security properties)
```

**Prompt pattern for security-auditor:** "Audit the changes in `[files]` for security vulnerabilities. These changes affect [token handling / cross-chain messaging / access control / etc]. The component's role is [description from context or knowledge graph]."

### After Confirming Important Information

**Trigger:** Any of:
- Architectural decision confirmed in conversation
- Domain knowledge learned from docs or user
- Integration detail clarified
- Protocol behavior confirmed

**Action:** Invoke `context-keeper`:

```
context-keeper: "Persist to group '[group_id]': [preprocessed factual statement]. 
Source: [conversation / docs / code review]."
```

Always preprocess the information into clear, entity-rich statements before delegating.

### When Assessing Project Status

**Trigger:** User asks about progress, what's left, blockers, or overall health. Also invoke proactively at the start of a major new task to understand context.

**Action:** Invoke `project-tracker`:

```
project-tracker: "Generate a project status report for the Stellar CCIP integration. 
Cross-reference Jira (NONEVM, label=stellar), recent Git history, and the knowledge graph. 
Focus on [specific area if applicable]."
```

### When Preparing Code for Review / PR

**Trigger:** User asks to prepare a PR, review code before merging, or do a final check.

**Action:** Run `quality-assurance-reviewer` and `security-auditor` in parallel:

```
┌────────────────────┐
│  Preparing for PR   │
└──────┬─────────────┘
       │ parallel
       ├──→ quality-assurance-reviewer  (quality gate)
       └──→ security-auditor            (security gate, if applicable)
```

## Invocation Patterns

### Parallel Invocation

When invoking multiple agents, **always use parallel Task calls in a single message** when the agents are independent:

```
Task(subagent_type="testing-engineer", prompt="...")
Task(subagent_type="security-auditor", prompt="...")
Task(subagent_type="quality-assurance-reviewer", prompt="...")
```

This runs all three simultaneously, saving significant time.

### Sequential Chaining

When one agent's output feeds another, chain them:

1. Run `security-auditor` → get findings
2. Run `testing-engineer` with: "Write tests that verify these security properties: [findings from auditor]"

### Prompt Quality Matters

Sub-agents start with **no conversation context**. Every prompt must be self-contained:

- **Specify files explicitly** — list the exact paths to review/test
- **Describe the feature** — what the code does and why
- **Set scope** — what to focus on, what to skip
- **Include constraints** — existing test patterns, coding conventions, language

Bad: "Review the recent changes."
Good: "Review `ccv/chain/chain.go` lines 284-540. This implements `DeployContractsForSelector` which deploys Stellar CCIP contracts for a given chain selector. The function was updated to use `ccipOffchain.EnvironmentTopology` instead of `deployments.EnvironmentTopology`. Focus on whether the topology resolution logic correctly handles missing signers and the placeholder fallback at line 545."

## Decision Flowchart

```
Did I just write or modify functional code?
├── Yes → Was it security-sensitive?
│   ├── Yes → testing-engineer + security-auditor + quality-assurance-reviewer + context-keeper
│   └── No  → testing-engineer + quality-assurance-reviewer + context-keeper
├── Did I just confirm an architectural decision or learn domain knowledge?
│   └── Yes → context-keeper
├── Is the user asking about project status, blockers, or planning?
│   └── Yes → project-tracker
├── Is the user preparing for a PR or review?
│   └── Yes → quality-assurance-reviewer + security-auditor (if security-relevant)
└── None of the above → No sub-agent needed
```

## What NOT to Delegate

- **Simple file reads or searches** — do these yourself, sub-agents add overhead
- **Single-line fixes** — not worth a testing or review cycle
- **Exploratory questions** — use `explore` sub-agent type or search directly
- **Config-only or doc-only changes** — no security audit needed, QA review optional

## Combining with Skills

Sub-agents reference these skills internally. You don't need to re-teach them, but mention relevant context:

- Agents that use Jira know the skill at `.cursor/skills/atlassian-jira-usage/SKILL.md`
- Agents that use the knowledge graph know `.cursor/skills/graphiti-mcp-usage/SKILL.md`
- Include group IDs (`ccip-architecture`, `stellar-integration`, `cctp-v2`, `audit-guide`) in prompts when relevant
