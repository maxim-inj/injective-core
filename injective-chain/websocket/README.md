# Injective Chain Stream WebSocket Server

The WebSocket server provides a real-time streaming interface for Injective chain events. It acts as a wrapper on top of the gRPC Chain Stream server, allowing clients to subscribe to blockchain updates using standard WebSocket connections instead of gRPC streams.

## Table of Contents

- [Injective Chain Stream WebSocket Server](#injective-chain-stream-websocket-server)
  - [Table of Contents](#table-of-contents)
  - [JSON Schemas](#json-schemas)
  - [Getting Started](#getting-started)
    - [Connecting to the WebSocket](#connecting-to-the-websocket)
    - [Subscribing to Events](#subscribing-to-events)
    - [Unsubscribing from Events](#unsubscribing-from-events)
    - [Available Filters](#available-filters)
    - [Response Format](#response-format)
  - [Configuration](#configuration)
    - [Configuration Options](#configuration-options)
    - [Example Configuration](#example-configuration)
  - [Current Limitations](#current-limitations)
    - [Security Considerations](#security-considerations)
      - [No Origin Validation](#no-origin-validation)
      - [No TLS/SSL Support](#no-tlsssl-support)
      - [No Authentication](#no-authentication)
      - [No Per-Client Rate Limiting](#no-per-client-rate-limiting)
    - [Operational Considerations](#operational-considerations)
      - [Subscription Identification](#subscription-identification)
      - [Connection Lifecycle](#connection-lifecycle)
      - [Resource Consumption](#resource-consumption)
      - [Default Unlimited Connections](#default-unlimited-connections)

---

## JSON Schemas

JSON Schema definitions are available for all WebSocket message types. These schemas follow the [JSON Schema Draft 2020-12](https://json-schema.org/draft/2020-12/schema) specification and can be used for request/response validation in client implementations.

| Schema | Description |
|--------|-------------|
| [subscribe_request.schema.json](./schemas/subscribe_request.schema.json) | Subscribe request with filter parameters |
| [unsubscribe_request.schema.json](./schemas/unsubscribe_request.schema.json) | Unsubscribe request with subscription ID |
| [success_response.schema.json](./schemas/success_response.schema.json) | Success response for subscribe/unsubscribe operations |
| [error_response.schema.json](./schemas/error_response.schema.json) | Error response format |
| [stream_response.schema.json](./schemas/stream_response.schema.json) | Stream data response containing chain events |

---

## Getting Started

### Connecting to the WebSocket

Connect to the WebSocket endpoint at:

```
ws://<node-address>:<port>/injstream-ws
```

**Example using JavaScript:**

```javascript
const ws = new WebSocket('ws://localhost:9998/injstream-ws');

ws.onopen = () => {
  console.log('Connected to Injective WebSocket');
};

ws.onmessage = (event) => {
  const response = JSON.parse(event.data);
  console.log('Received:', response);
};

ws.onerror = (error) => {
  console.error('WebSocket error:', error);
};

ws.onclose = () => {
  console.log('Disconnected from WebSocket');
};
```

### Subscribing to Events

Send a JSON-RPC 2.0 request with the `subscribe` method. The request must include:

- `jsonrpc`: Always `"2.0"`
- `id`: A positive integer that will be used to identify responses for this subscription
- `method`: `"subscribe"`
- `params.req.subscription_id`: A **client-provided unique identifier** for this subscription (used for unsubscribing)
- `params.req.filter`: An object containing your subscription filters

**Example: Subscribe to Oracle Prices**

```javascript
const subscribeRequest = {
  jsonrpc: '2.0',
  id: 1,
  method: 'subscribe',
  params: {
    req: {
      subscription_id: 'my-oracle-prices',
      filter: {
        oracle_price_filter: {
          symbol: ['*']  // Use '*' to subscribe to all symbols
        }
      }
    }
  }
};

ws.send(JSON.stringify(subscribeRequest));
```

**Example: Subscribe to Spot Orders for a Specific Market**

```javascript
const subscribeRequest = {
  jsonrpc: '2.0',
  id: 2,
  method: 'subscribe',
  params: {
    req: {
      subscription_id: 'spot-orders-market-1',
      filter: {
        spot_orders_filter: {
          market_ids: ['0x0611780ba69656949525013d947713300f56c37b6175e02f26bffa495c3208fe'],
          subaccount_ids: ['*']
        }
      }
    }
  }
};

ws.send(JSON.stringify(subscribeRequest));
```

**Example: Subscribe to Multiple Event Types**

```javascript
const subscribeRequest = {
  jsonrpc: '2.0',
  id: 3,
  method: 'subscribe',
  params: {
    req: {
      subscription_id: 'my-trading-stream',
      filter: {
        spot_orders_filter: {
          market_ids: ['*'],
          subaccount_ids: ['0xeb8cf88b739fe12e303e31fb88fc37751e17cf3d000000000000000000000000']
        },
        derivative_orders_filter: {
          market_ids: ['*'],
          subaccount_ids: ['0xeb8cf88b739fe12e303e31fb88fc37751e17cf3d000000000000000000000000']
        },
        oracle_price_filter: {
          symbol: ['BTCUSD', 'ETHUSD']
        },
        positions_filter: {
          subaccount_ids: ['0xeb8cf88b739fe12e303e31fb88fc37751e17cf3d000000000000000000000000'],
          market_ids: ['*']
        }
      }
    }
  }
};

ws.send(JSON.stringify(subscribeRequest));
```

**Successful Subscription Response:**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": "success"
}
```

After a successful subscription, you will start receiving stream updates with the same `id`.

### Unsubscribing from Events

To unsubscribe, send a request with the `unsubscribe` method and the **subscription_id** you used when subscribing:

```javascript
const unsubscribeRequest = {
  jsonrpc: '2.0',
  id: 100,  // This can be any valid ID for the RPC request
  method: 'unsubscribe',
  params: {
    req: {
      subscription_id: 'my-oracle-prices'
    }
  }
};

ws.send(JSON.stringify(unsubscribeRequest));
```

**Successful Unsubscribe Response:**

```json
{
  "jsonrpc": "2.0",
  "id": 100,
  "result": "success"
}
```

> **Note:** The `subscription_id` must match exactly the ID you provided when creating the subscription.

### Available Filters

You can subscribe to any combination of the following event types. At least one filter must be specified.

| Filter | Description | Parameters |
|--------|-------------|------------|
| `bank_balances_filter` | Bank balance changes | `accounts`: List of account addresses |
| `subaccount_deposits_filter` | Subaccount deposit changes | `subaccount_ids`: List of subaccount IDs |
| `spot_trades_filter` | Spot market trades | `market_ids`, `subaccount_ids` |
| `derivative_trades_filter` | Derivative market trades | `market_ids`, `subaccount_ids` |
| `spot_orders_filter` | Spot order updates | `market_ids`, `subaccount_ids` |
| `derivative_orders_filter` | Derivative order updates | `market_ids`, `subaccount_ids` |
| `spot_orderbooks_filter` | Spot orderbook updates | `market_ids` |
| `derivative_orderbooks_filter` | Derivative orderbook updates | `market_ids` |
| `positions_filter` | Position updates | `subaccount_ids`, `market_ids` |
| `oracle_price_filter` | Oracle price updates | `symbol`: List of price symbols |
| `order_failures_filter` | Order failure notifications | `accounts`: List of account addresses |
| `conditional_order_trigger_failures_filter` | Conditional order trigger failures | `subaccount_ids`, `market_ids` |

**Wildcard Support:**

Use `"*"` to match all values for a parameter:

```json
{
  "market_ids": ["*"],
  "subaccount_ids": ["*"]
}
```

### Response Format

Stream responses are sent as JSON-RPC responses with the following structure:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "block_height": "12345678",
    "block_time": "1702123456789",
    "bank_balances": [...],
    "subaccount_deposits": [...],
    "spot_trades": [...],
    "derivative_trades": [...],
    "spot_orders": [...],
    "derivative_orders": [...],
    "spot_orderbook_updates": [...],
    "derivative_orderbook_updates": [...],
    "positions": [...],
    "oracle_prices": [...],
    "gas_price": "160000000",
    "order_failures": [...],
    "conditional_order_trigger_failures": [...]
  }
}
```

Each response contains updates for a single block. Only fields matching your subscription filters will contain data; others will be empty arrays or omitted.

---

## Configuration

The WebSocket server is configured in the `app.toml` configuration file under the `[injective-websocket]` section.

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `address` | string | `""` (disabled) | The address and port to bind the WebSocket server. Set to empty string to disable the server. Example: `"0.0.0.0:9998"` |
| `max-open-connections` | int | `0` (unlimited) | Maximum number of simultaneous WebSocket connections. Set to `0` for unlimited connections. **Recommended:** Set a reasonable limit (e.g., `100-1000`) in production. |
| `read-timeout` | duration | `10s` | Maximum time to wait for reading a complete request from the client. Connections that exceed this timeout will be closed. |
| `write-timeout` | duration | `10s` | Maximum time to wait for writing a response to the client. Slow consumers may be disconnected if writes consistently timeout. |
| `max-body-bytes` | int64 | `1000000` (1MB) | Maximum allowed size for the HTTP request body in bytes. Requests exceeding this limit will be rejected. |
| `max-header-bytes` | int | `1048576` (1MB) | Maximum allowed size for HTTP headers in bytes. Requests with headers exceeding this limit will be rejected. |
| `max-request-batch-size` | int | `10` | Maximum number of RPC calls allowed in a single batch request. |

### Example Configuration

Add the following to your `app.toml`:

```toml
###############################################################################
###                  Injective Websocket Configuration                      ###
###############################################################################

[injective-websocket]

# Address defines the websocket server address to bind to.
# Leave empty to disable the websocket server.
address = "0.0.0.0:9998"

# MaxOpenConnections sets the maximum number of simultaneous connections.
# Set to 0 for unlimited (not recommended for production).
max-open-connections = 100

# ReadTimeout defines the HTTP read timeout.
read-timeout = "10s"

# WriteTimeout defines the HTTP write timeout.
write-timeout = "10s"

# MaxBodyBytes defines the maximum allowed HTTP body size (in bytes).
max-body-bytes = 1000000

# MaxHeaderBytes defines the maximum allowed HTTP header size (in bytes).
max-header-bytes = 1048576

# MaxRequestBatchSize defines the maximum number of RPC calls per batch request.
max-request-batch-size = 10
```

**Prerequisites:**

The WebSocket server requires the Chain Stream server to be enabled. Ensure the following is also configured:

```toml
# Enable the chainstream gRPC server
chainstream-server = "0.0.0.0:9999"

# Buffer capacities for the stream server
chainstream-server-buffer-capacity = 100
chainstream-publisher-buffer-capacity = 100
```

---

## Current Limitations

### Security Considerations

The current WebSocket implementation has certain security limitations that should be addressed through infrastructure configuration when deploying in production environments.

#### No Origin Validation

The WebSocket server accepts connections from **any origin**. This makes it potentially vulnerable to Cross-Site WebSocket Hijacking (CSWSH) attacks if exposed directly to the internet.

**Mitigation:** Deploy behind a reverse proxy (e.g., nginx, Traefik, Caddy) that validates the `Origin` header:

```nginx
# Example nginx configuration
location /injstream-ws {
    # Only allow specific origins
    if ($http_origin !~* "^https://(app\.injective\.network|your-domain\.com)$") {
        return 403;
    }
    
    proxy_pass http://127.0.0.1:9998;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
}
```

#### No TLS/SSL Support

The WebSocket server only supports unencrypted `ws://` connections. All traffic is transmitted in plaintext.

**Mitigation:** Use a TLS-terminating reverse proxy to provide `wss://` (WebSocket Secure) connections:

```nginx
# Example nginx TLS termination
server {
    listen 443 ssl;
    server_name your-domain.com;
    
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;
    
    location /injstream-ws {
        proxy_pass http://127.0.0.1:9998;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

#### No Authentication

The WebSocket server does not implement authentication. Any client that can reach the server can subscribe to any available data.

**Mitigation:** If authentication is required:
- Use a reverse proxy with authentication middleware
- Implement API key validation at the proxy level
- Use network-level access controls (firewalls, VPNs)

#### No Per-Client Rate Limiting

While `max-open-connections` limits total connections, there is no built-in rate limiting per client IP. A single client can create multiple subscriptions.

**Mitigation:** Implement rate limiting at the reverse proxy level:

```nginx
# Example nginx rate limiting
limit_conn_zone $binary_remote_addr zone=ws_conn:10m;
limit_conn ws_conn 10;  # Max 10 connections per IP
```

### Operational Considerations

#### Subscription Identification

Subscriptions are identified by a **client-provided `subscription_id`**. This approach provides:

- **Simple unsubscribe**: Just reference the `subscription_id` you provided when subscribing
- **Predictable behavior**: No dependency on JSON serialization details
- **Client control**: You manage your own subscription identifiers
- **Reconnection support**: Re-establish subscriptions with the same IDs after reconnect

**Important considerations:**
- The `subscription_id` must be unique within your connection
- Attempting to create a subscription with an existing ID will fail
- Different clients (connections) can use the same `subscription_id` independently

#### Connection Lifecycle

- When a WebSocket connection is closed, all associated subscriptions are automatically cancelled
- The server sends ping frames periodically to detect stale connections
- Clients should implement reconnection logic with exponential backoff

#### Resource Consumption

Each subscription spawns a goroutine that consumes from the chain event bus. Consider:

- Limiting the number of subscriptions per client at the infrastructure level
- Monitoring server memory and CPU usage
- Using specific filters rather than wildcards (`*`) when possible to reduce data volume

#### Default Unlimited Connections

The default value for `max-open-connections` is `0` (unlimited). For production deployments, always set an appropriate limit based on your server capacity and expected client load.

