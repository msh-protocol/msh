# msh-fleet: Enterprise Control Plane

The `msh-fleet` sub-system is the centralized command-and-control registry for managing distributed `msh` agent daemons. It enables secure, real-time observability and NAT-bypassing terminal streaming for thousands of concurrent AI agents.

## Architecture

The system consists of two primary components:
1. **The Fleet Hub (`msh fleet start`)**: A lightweight, centralized WebSocket server and UI dashboard. It acts as the registry for all daemons.
2. **The Daemons (`msh serve --fleet <url>`)**: The worker nodes where AI agents execute commands. They connect *outbound* to the Fleet Hub, bypassing inbound firewalls and NATs.

## Quick Start

### 1. Start the Fleet Hub
Run the following command on your central server (or locally):
```bash
msh fleet start
```
*Outputs:*
```
[msh-fleet] Generated Admin Token: msh-1234567890
msh fleet hub listening on http://127.0.0.1:9000
```
*Keep this token safe! It is required to connect daemons and access the dashboard.*

### 2. Connect a Daemon
On your worker machines, start the `msh` daemon and point it to your Fleet Hub using the generated token:
```bash
msh serve --port 8080 --fleet ws://127.0.0.1:9000 --token msh-1234567890
```
The daemon will automatically register itself with the hub and establish a persistent, bidirectional WebSocket tunnel.

### 3. Access the Dashboard
Open `http://127.0.0.1:9000` in your browser. 
You will be prompted for your Admin Token. Once authenticated, you will see a real-time list of all connected daemons.

## Features

### Reverse NAT Tunneling
Because daemons connect *outbound* to the Fleet Hub via WebSockets, you do not need to open inbound ports on your worker machines. The Fleet Hub can route commands and stream logs back to the dashboard through this established tunnel.

### Live Terminal Streaming
Click the **Live Terminal** button on any connected node in the dashboard. The Fleet Hub will instantly instruct the daemon to begin tailing its execution logs (`.msh/daemons/<id>.log`) and stream them directly to your browser with zero latency.

### Secure by Default
- The Fleet Hub verifies the `Authorization` token for every WebSocket connection and API request.
- Cross-Origin Resource Sharing (CORS) is strictly managed.
- Embedded UI assets are served directly from the single Go binary with strict Cache-Control headers.
