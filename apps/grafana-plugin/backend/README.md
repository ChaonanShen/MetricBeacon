# Grafana Plugin Backend

The backend is a thin Grafana Plugin SDK resource layer. It will construct RequestContext
from Grafana context and proxy generated AI Core client calls. It must not persist product
data, run workflows, call MCP directly or read Mock fixtures.

G0 intentionally contains only a buildable command. The SDK and generated AI Core client
are introduced by G5 after the corresponding contracts exist.
