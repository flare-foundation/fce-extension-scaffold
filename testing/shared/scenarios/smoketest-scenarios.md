# Smoketest Scenarios

Pick one per cycle. Rotate sequentially, then loop.

## Scenario List

1. **full-setup-standard** — Run `./scripts/full-setup.sh --test`. Default config. Verify all 4 phases pass.
2. **step-by-step** — Run each phase separately: `pre-build.sh`, then `start-services.sh`, then `post-build.sh`, then `test.sh`. Verify each exits 0.
3. **rapid-cycle** — Run full-setup --test, teardown, immediately run full-setup --test again. Verify second run also passes.
4. **verify-between-phases** — Run pre-build.sh. Run `verify-deploy --step deploy`. Run start-services.sh. Run `verify-deploy --step services`. Continue for each phase.
5. **unicode-payload** — Run full setup. Before test.sh, modify the test payload name to unicode characters. Verify extension handles it.
6. **long-name-payload** — Run full setup. Use a 1000-character name string in test payload. Verify behavior.
7. **empty-payload** — Run full setup. Use empty string "" as name. Verify extension handles it gracefully.
8. **double-test** — Run full setup. Run test.sh twice in a row without teardown. Verify second run also passes (tests should be idempotent).
9. **verify-only** — Run full setup. Then run `verify-deploy` with no --step flag (all checks). Verify everything passes.
