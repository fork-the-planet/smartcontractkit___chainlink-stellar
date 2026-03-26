---
name: atlassian-jira-usage
description: "Interact with Jira for the Stellar CCIP integration project using the Atlassian MCP. Provides project key (NONEVM), board ID (6688), and required label (stellar) so the agent can create, search, and manage tickets without unnecessary lookups. Use when creating Jira tickets, epics, searching issues, triaging bugs, generating reports, or any Jira/Atlassian operation for this project."
---

# Atlassian Jira Usage — Stellar Integration

## Project Constants

Use these values directly. **Never search for or ask the user about them.**

| Field | Value |
|---|---|
| **Project Key** | `NONEVM` |
| **Board ID** | `6688` |
| **Required Label** | `stellar` |

## Label Requirement

**Every ticket, epic, story, task, or bug created in NONEVM must include the `stellar` label.** This is how the board filters work — tickets without this label will not appear on the Stellar board.

When calling `createJiraIssue`, always include:

```
additional_fields={
  "labels": ["stellar"]
}
```

When adding labels to an existing issue, preserve any existing labels and append `stellar` if missing.

## Creating Tickets

Use `createJiraIssue` from the Atlassian MCP. Always provide:

- `projectKey`: `"NONEVM"`
- `labels`: `["stellar"]` via `additional_fields`

```
createJiraIssue(
  cloudId="...",
  projectKey="NONEVM",
  issueTypeName="Task",        # or Story, Bug, Epic
  summary="...",
  description="...",
  additional_fields={
    "labels": ["stellar"]
  }
)
```

For epics with child tickets, create the epic first, capture its key, then create children with `parent` set to the epic key. Every child also needs the `stellar` label.

## Searching Issues

Pre-built JQL patterns for this project. Replace placeholders as needed.

**All open Stellar issues:**
```jql
project = NONEVM AND labels = stellar AND status != Done ORDER BY priority DESC, updated DESC
```

**Stellar issues by status:**
```jql
project = NONEVM AND labels = stellar AND status = "In Progress" ORDER BY updated DESC
```

**Recently completed:**
```jql
project = NONEVM AND labels = stellar AND status = Done AND resolved >= -7d ORDER BY resolved DESC
```

**Blocked or high-priority:**
```jql
project = NONEVM AND labels = stellar AND (priority IN (Highest, High) OR status = Blocked) AND status != Done ORDER BY priority DESC
```

**Issues under an epic:**
```jql
parent = "NONEVM-XXX" AND labels = stellar ORDER BY priority DESC
```

**Text search within Stellar scope:**
```jql
project = NONEVM AND labels = stellar AND text ~ "search terms" ORDER BY updated DESC
```

**Unassigned Stellar issues:**
```jql
project = NONEVM AND labels = stellar AND assignee is EMPTY AND status != Done ORDER BY priority DESC
```

## Triaging Bugs

When triaging errors or bugs for this project, scope duplicate searches to the Stellar label:

```jql
project = NONEVM AND labels = stellar AND type = Bug AND text ~ "error signature" ORDER BY created DESC
```

Always create new bugs with `issueTypeName="Bug"` and include the `stellar` label.

## Status Reports

When generating status reports, use the Stellar-scoped JQL patterns above. Group by status:

1. Query completed: `status = Done AND resolved >= -7d`
2. Query in-progress: `status = "In Progress"`
3. Query blocked: `status = Blocked`
4. Query high-priority open: `priority IN (Highest, High) AND status != Done`

All queries should include `project = NONEVM AND labels = stellar`.

## Quick Reference

| Action | Key Info |
|---|---|
| Create any issue | `projectKey="NONEVM"`, labels=`["stellar"]` |
| Search issues | `project = NONEVM AND labels = stellar` in JQL |
| Board | ID `6688` |
| Epic children | Set `parent` to epic key, still add `stellar` label |
| Bug triage | Scope search to `labels = stellar AND type = Bug` |
