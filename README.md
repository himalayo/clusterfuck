# clusterfuck
> An experimental distributed game backend inspired by Habbo

> **About the name:**
> Distributed, real-time game servers are inherently complex systems with competing constraints around latency, consistency, and scalability.
> This project intentionally explores that complexity rather than hiding it, hence the name.

clusterfuck is an experimental, distributed backend architecture for a Habbo-like game, built as a learning and exploration project.
The focus of this repository is not feature completeness or short-term usability, but architectural tradeoffs around:
1. low-latency networking
2. event-driven microservices
3. inter-service communication
4. explicit concurrency and failure recovery
5. realistic constraints of MMO-style servers

This project is heavily inspired by publicly available information about Habbo’s backend evolution,
especially their transition from tightly coupled systems to message-based architectures.

----------------------

## Architecture overview
- Networking service
  - WebSocket-based
  - Single-session authority
  - Latency-critical
  - Intentionally more coupled

- Player service
  - Session state
  - Login flow
  - Core gameplay events
  - In-process event bus for deterministic handling

- Auxiliary services (e.g. achievements)
  - Eventually consistent
  - React to player lifecycle events via Redis Streams (or gRPC in some instances)

Currently, all of the services were written in Go. This might change in the future if necessary.

One of the key decisions for a microservices architecture, besides the possibility to scale horizontally,
was precisely more flexibility in the choice of programming language for different parts of the emulator.

Horizontal scalability is the primary goal. As such, services must be as stateless and idempotent as possible.

## Contributions
At this stage, this repository is primarily a personal learning project.
Feedback, discussion, and architectural critique are welcome, but the internal design is still in flux and may change significantly.
