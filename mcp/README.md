# markupp-mcp

MCP server for [markupp](https://github.com/markupp-labs/markupp). It lets AI agents such as
Claude Code, Codex, Cursor and VS Code read and write your markupp notes, follow wikilinks
and manage tenants and invites, always through the markupp REST API and with your own
permissions.

Requires Node.js 22.12 or later.

## Install

Sign in once. The command opens your browser on the markupp instance and stores the login
in `$XDG_CONFIG_HOME/markupp/credentials.json` (or `~/.config/markupp/credentials.json`):

```sh
npx -y markupp-mcp login https://your-markupp-instance
```

Then register the server in your agent as a stdio command. With Claude Code:

```sh
claude mcp add markupp -- npx -y markupp-mcp
```

Any other client that takes a JSON config:

```json
{
  "mcpServers": {
    "markupp": {
      "command": "npx",
      "args": ["-y", "markupp-mcp"]
    }
  }
}
```

## Commands

```text
markupp-mcp               serve MCP over stdio with the stored login
markupp-mcp login [url]   sign in through the browser (default http://localhost:8080)
markupp-mcp logout        revoke and delete the stored login
markupp-mcp http          serve MCP over Streamable HTTP (self-hosted deployments)
```

The `http` mode is what a markupp instance runs next to its API. Agents connect to it by
URL with OAuth, so no local install is needed; see the
[repository](https://github.com/markupp-labs/markupp) for self-hosting.

## License

AGPL-3.0-only
