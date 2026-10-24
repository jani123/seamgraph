# ADR 1: deterministic local adapters

Accepted. Use integer cents and in-memory stock, payment and shipping adapters.
No credentials, broker or external runtime services are required. This keeps
review histories reproducible but omits persistence, concurrency, crash recovery
and real delivery guarantees. HTTP and messages are explicit integration contracts;
static cross-language call discovery is outside the current adapters.
