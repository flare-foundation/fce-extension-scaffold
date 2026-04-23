# Chaos Scenarios

Pick or invent. You can modify code in your worktree. Be creative.

## Runtime Scenarios (no code changes)

1. **concurrent-prebuild** — Run two `pre-build.sh` processes in parallel (background one, run the other). Both do on-chain operations. Record: do both succeed? Any conflicts?
2. **kill-mid-registration** — Run start-services.sh, then start post-build.sh. After 5 seconds, run `docker kill extension-tee`. Record: does post-build fail cleanly? Can it resume?
3. **wrong-phase-order** — Run test.sh before post-build.sh. Record error. Then run post-build.sh before start-services.sh. Record error. How clear are the messages?
4. **corrupt-extension-env** — Run pre-build.sh. Replace EXTENSION_ID in config/extension.env with "GARBAGE_VALUE". Run start-services.sh. Record behavior.
5. **rapid-instructions** — Run full setup. Then send 10 instructions in quick succession (loop calling run-test or go run cmd/run-test). Record: do all complete? Any dropped?
6. **double-register-sender** — Run pre-build.sh to get a contract. Save the INSTRUCTION_SENDER. Manually call register-extension again with the same INSTRUCTION_SENDER. Record: does the registry allow it? What happens to setExtensionId?
7. **no-teardown-redeploy** — Run full-setup --test. Do NOT run stop-services.sh. Run full-setup --test again. Record: does docker compose handle it? Port conflicts?
8. **key-swap-between-phases** — Run pre-build.sh with KEY_A. Change DEPLOYMENT_PRIVATE_KEY to KEY_B (a different funded key). Run start-services.sh + post-build.sh. Record: does it fail because KEY_B isn't the extension owner?
9. **stale-services** — Run start-services.sh. Wait 5 minutes (let proxy cycle). Run test.sh. Expected: should still work. Record any staleness issues.

## Code Modification Scenarios (use worktree)

10. **skip-preflight** — In worktree, edit `scripts/pre-build.sh` to remove the preflight check call. Run pre-build.sh with wrong addresses. Record: what happens without the safety net?
11. **hash-mismatch** — In worktree, edit `internal/config/config.go` to change an OPType constant (e.g., "GREETING" to "greeting"). Rebuild. Run full-setup --test. Record: does the 501 error clearly explain the hash mismatch?
12. **wrong-dockerfile-path** — In worktree, edit `docker-compose.yaml` to reference a wrong Dockerfile path. Run start-services.sh. Record error clarity.
13. **wrong-ports** — In worktree, change EXTENSION_PORT in docker-compose.yaml to 9999. Run start-services.sh + test.sh. Record: does the error explain the port mismatch?
14. **wrong-fee** — In worktree, change the hardcoded instruction fee in `tools/pkg/utils/instructions.go` (change 1000000 to 1). Run test.sh. Record revert message clarity.
15. **encoding-swap** — In worktree, change the SAY_HELLO handler to use ABI decoding instead of JSON. Rebuild. Run test.sh. Record: is the decoding error clear?
16. **remove-waitmined-timeout** — In worktree, add a context.WithTimeout of 1 second to a WaitMined call. Run pre-build.sh. Record: does it timeout? What happens?
17. **solidity-case-change** — In worktree, change `bytes32("GREETING")` to `bytes32("Greeting")` in InstructionSender.sol. Run generate-bindings.sh, rebuild, run full-setup --test. Record: is the hash mismatch caught?

## Invention

After running through the above, invent new scenarios. Look at:
- Previous test results in results/ — patterns of failure
- The edge case docs in notes/ — anything not covered above
- Combinations of failures (e.g., wrong key + wrong port + wrong encoding)
