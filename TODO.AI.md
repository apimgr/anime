# AI.md issues

- [ ] AI.md PART 5/8 maps the listen-address environment variable to `LISTEN`, while PART 26's Docker entrypoint and environment examples use `ADDRESS`; PART 26 also uses `LOGS_DIR` where PART 5 defines `LOG_DIR`. Establish one canonical mapping and align the entrypoint, container examples, and configuration table.
- [ ] AI.md PART 13 around lines 2269–2272 says to include metrics status in public healthz, then says NEVER to include metrics status. Define one rule for metrics status in health responses.
- [ ] AI.md PART 13 around lines 2255–2257 requires public health to include version, uptime, Tor address, and request counts, while PART 13 around line 5470 limits `/server/healthz` to status only with no connection details. Define the complete public health response fields and align the health contract and examples.
