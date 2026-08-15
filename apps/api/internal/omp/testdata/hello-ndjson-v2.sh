#!/usr/bin/env bash
echo '{"protocol_version":2,"omp_version":"omp/17.3.4"}'
echo '{"type":"agent_start","seq":1}'
echo '{"type":"delta","seq":2,"text":"hello"}'
echo '{"type":"agent_end","seq":3}'
# Stay alive briefly so the SSE stream still has time to read.
sleep 5
