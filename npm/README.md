# trvl-mcp

AI travel agent MCP server. 1 smart MCP tool plus 64 compatibility aliases for flights, hotels, rental cars, ground transport, price alerts. No API keys required for core search.

## Usage

```
npx trvl-mcp
```

## Claude Desktop

Add to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "trvl": {
      "command": "npx",
      "args": ["trvl-mcp"]
    }
  }
}
```

## What's included

- Flight search (Google Flights, Kiwi)
- Hotel search (Google Hotels, Booking.com, Airbnb, Hostelworld, Trivago)
- Ground transport (buses, trains, ferries — 20 providers)
- Destination intelligence (weather, safety, holidays, events)
- Trip planning and price alerts
- Travel hacks detection (37 detectors)

## License

PolyForm Noncommercial 1.0.0
