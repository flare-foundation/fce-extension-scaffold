# Edge Case Scenarios

Work through sequentially. Track progress in scenario-tracker.md.
Each scenario tests a specific failure mode from the edge case docs.

## Contract Deployment (D-series)

- **D1-wrong-registry** — Set ADDRESSES_FILE to a file with wrong TeeExtensionRegistry address (a random valid address that is an EOA). Run pre-build.sh. Expected: deployment succeeds but registration fails. Record error message clarity.
- **D3-zero-addresses** — Create a temp addresses file with zero addresses for registries. Run pre-build.sh. Expected: failure. Record error message.
- **D4-double-deploy** — Run pre-build.sh twice in a row. Expected: second run creates new contracts/extension. Record: does it warn about existing config/extension.env?
- **D5-wrong-key** — Temporarily set DEPLOYMENT_PRIVATE_KEY to a valid but unfunded key (generate one). Run pre-build.sh. Expected: "insufficient funds" error. Record error clarity.
- **D7-output-capture** — Run deploy-contract manually and check that the captured address is a valid `0x[0-9a-fA-F]{40}` format. No garbage in output.

## Registration (R-series)

- **R3-partial-failure** — This is hard to trigger deliberately. Instead: run pre-build.sh, verify extension.env has valid values. Then run verify-deploy --step register. Record all checks pass.
- **R5-rerun-after-success** — Run pre-build.sh, then run it again. Expected: "key already supported" or similar. Record: does it handle re-run gracefully?
- **R7-duplicate-sender** — Run pre-build.sh. Save the INSTRUCTION_SENDER. Run pre-build.sh again (deploys new contract but registers new extension). Now two extensions exist. Run test.sh. Record: which extension does setExtensionId find?

## Service Startup (S-series)

- **S1-missing-extension-id** — Delete config/extension.env. Run start-services.sh. Expected: clear error "EXTENSION_ID not set". Record error message.
- **S2-stale-extension-id** — Edit config/extension.env to set EXTENSION_ID to a random hex value. Run start-services.sh then test.sh. Expected: tests fail (instructions go to wrong extension). Record: is the failure clear?
- **S3-no-docker-network** — Remove the external docker_default network (if safe). Run start-services.sh. Expected: clear Docker error. Record message.
- **S9-wrong-proxy-url** — Set EXT_PROXY_URL to a non-existent URL (e.g., http://localhost:9999). Run test.sh. Expected: connection refused. Record error clarity.

## TEE Version (V-series)

- **V1-already-registered** — Run full setup. Run post-build.sh again (allow-tee-version only). Expected: "version already registered, skipping". Record: handled gracefully?
- **V6-proxy-not-running** — Stop services. Run allow-tee-version manually. Expected: connection error. Record error message.

## TEE Machine (T-series)

- **T4-already-registered** — Run full setup. Run register-tee again. Expected: revert or graceful skip. Record error message clarity.

## Testing (E-series)

- **E1-no-extension-registered** — Deploy contract only (no register-extension). Run test.sh with the contract address. Expected: setExtensionId fails. Record error message.
- **E4-result-polling** — Run full setup --test. Check timing: how long does result polling take? Record average.

## Verify-Deploy Validation

- **verify-before-prebuild** — Run verify-deploy before any setup. Expected: appropriate FAILs for missing config. Record check coverage.
- **verify-after-prebuild** — Run pre-build.sh, then verify-deploy --step deploy --step register. Expected: PASSes. Record.
- **verify-after-start** — Run through start-services.sh, then verify-deploy --step services. Expected: PASSes. Record.
- **verify-full-after-setup** — Run full-setup --test, then verify-deploy (all checks). Expected: all PASS. Record.
