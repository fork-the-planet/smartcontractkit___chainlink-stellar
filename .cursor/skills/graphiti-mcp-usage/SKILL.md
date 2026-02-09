---
name: graphiti-mcp-usage
description: Reference for using the Graphiti MCP knowledge graph (graphiti-mcp-aws) to add, search, and manage project memories. Use when interacting with the knowledge graph, adding memories, searching for context, or managing graph data.
---

# Graphiti MCP Usage

Quick reference for the `graphiti-mcp-aws` MCP server tools. Use this when persisting or retrieving project knowledge.

## Tools Overview

| Tool | Purpose |
|------|---------|
| `add_memory` | Add an episode (text, JSON, or message) to the graph |
| `search_nodes` | Find entities by natural language query |
| `search_memory_facts` | Find relationships/facts between entities |
| `get_episodes` | Retrieve recent episodes by group |
| `get_entity_edge` | Get a specific relationship by UUID |
| `delete_entity_edge` | Remove a relationship |
| `delete_episode` | Remove an episode |
| `clear_graph` | Wipe all data for a group (destructive) |
| `get_status` | Check server/database health |

## Adding Memories

### `add_memory` Parameters

| Parameter | Required | Description |
|-----------|----------|-------------|
| `name` | Yes | Short descriptive title for the episode |
| `episode_body` | Yes | The content to persist (see formatting below) |
| `group_id` | No | Namespace for the memory (defaults to server default) |
| `source` | No | `"text"` (default), `"json"`, or `"message"` |
| `source_description` | No | Where the information came from |

### Choosing `source` Type

- **`text`**: Narrative descriptions, architectural explanations, summaries
- **`json`**: Structured data with clear entity-relationship mappings. Must be a valid JSON string.
- **`message`**: Conversation-style content

### Writing Effective Episode Bodies

The episode body is what Graphiti processes to extract entities and relationships. Quality in = quality out.

**Do:**
- Name entities explicitly: "The CommitStore contract on Ethereum mainnet"
- State relationships clearly: "OnRamp sends messages to the CommitStore"
- Include relevant attributes: "The Stellar token has 7 decimal places"
- Use consistent terminology across episodes

**Don't:**
- Use vague pronouns: "it connects to the other thing"
- Add noise: "as we discussed earlier, I think maybe..."
- Dump raw docs without distilling

### JSON Source Example

When adding structured data, pass a JSON string:

```
add_memory(
  name="Stellar Contract Addresses",
  episode_body='{"contracts": [{"name": "OnRamp", "address": "CA...", "network": "stellar-testnet"}, {"name": "TokenPool", "address": "CB...", "network": "stellar-testnet"}]}',
  source="json",
  group_id="stellar-integration",
  source_description="deployment config"
)
```

## Group IDs

Group IDs namespace the graph. Always use the correct one:

| Group ID | Use For |
|----------|---------|
| `ccip-architecture` | CCIP protocol design, cross-chain messaging, lane configs |
| `stellar-integration` | Stellar/Soroban contracts, SDK usage, chain-specific details |
| `cctp-v2` | CCTP v2 protocol, token transfer mechanics |

When information belongs to multiple domains, add it to each relevant group with appropriately tailored episode bodies.

## Searching the Graph

### `search_nodes` — Find Entities

Use for: "What do we know about X?"

```
search_nodes(query="Stellar OnRamp contract", group_ids=["stellar-integration"], max_nodes=5)
```

- Returns entity nodes with names, types, and summaries
- Use `entity_types` to filter (e.g., `["Contract", "Protocol"]`)

### `search_memory_facts` — Find Relationships

Use for: "How does X relate to Y?" or "What facts do we have about X?"

```
search_memory_facts(query="how does OnRamp communicate with CommitStore", group_ids=["ccip-architecture"], max_facts=10)
```

- Returns edges between entities with fact descriptions and temporal metadata
- Use `center_node_uuid` to explore around a specific entity (get UUID from `search_nodes` first)

### Search Tips

1. **Be specific**: "Stellar USDC decimal precision" beats "token details"
2. **Use group filters**: Always pass `group_ids` to narrow results
3. **Chain searches**: Use `search_nodes` to find an entity, then `search_memory_facts` with `center_node_uuid` to explore its relationships
4. **Check before adding**: Search first to avoid duplicates

## Preprocessing Guidelines

Your model is more capable than what runs inside Graphiti. Preprocess data before adding:

1. **Distill**: Convert verbose docs into concise factual statements
2. **Normalize**: Use consistent entity names (check existing nodes first)
3. **Decompose**: Split complex topics into multiple focused memories
4. **Enrich**: Add context that makes the memory self-contained
5. **Deduplicate**: Search the graph before adding to avoid redundancy

## Maintenance

- **Superseded facts**: When adding information that updates a previous fact, mention this explicitly so Graphiti tracks temporal validity
- **Deleting**: Use `delete_entity_edge` or `delete_episode` to remove incorrect data. Get UUIDs from search results.
- **Never use `clear_graph`** unless explicitly asked — it's destructive and irreversible
