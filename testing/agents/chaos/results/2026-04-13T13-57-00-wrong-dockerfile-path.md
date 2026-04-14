# [Chaos] Wrong Dockerfile Path
**Date:** 2026-04-13 13:57:00
**Scenario:** wrong-dockerfile-path (#12)
**Duration:** 1m 30s
**Result:** INTERESTING
**Code Modified:** YES

## Modifications Made
```diff
diff --git a/docker-compose.yaml b/docker-compose.yaml
index f67aa39..449439f 100644
--- a/docker-compose.yaml
+++ b/docker-compose.yaml
@@ -35,8 +35,8 @@ services:
 
   extension-tee:
     build:
-      context: ../..
-      dockerfile: extension-examples/extension-scaffold/Dockerfile
+      context: /home/snojj25/tee
+      dockerfile: extension-examples/extension-scaffold/Dockerfile.WRONG
     env_file:
       - path: ./config/extension.env
         required: false
@@ -54,8 +55,8 @@ services:
 
   types-server:
     build:
-      context: ../..
-      dockerfile: extension-examples/extension-scaffold/Dockerfile
+      context: /home/snojj25/tee
+      dockerfile: extension-examples/extension-scaffold/Dockerfile.WRONG
     command: ["./types-server"]
```

## What Was Tried
1. Changed both `dockerfile:` values in `docker-compose.yaml` from `Dockerfile` to `Dockerfile.WRONG`.
2. Created a fake `config/extension.env` with an EXTENSION_ID to bypass the pre-build check.
3. Ran `start-services.sh`.

**Note:** Initially tested with the original `context: ../..` — but from the worktree location (`testing/agents/chaos/worktree/`), `../..` resolves to `testing/agents/` rather than the repo root. Docker failed with a misleading path error (`lstat .../testing/agents/extension-examples: no such file or directory`) before even reaching the Dockerfile lookup. Had to fix the context to an absolute path to isolate the actual Dockerfile error.

## What Happened

### Test A: Wrong context (original `../..` from worktree)
```
resolve : lstat /home/snojj25/tee/extension-examples/extension-scaffold/testing/agents/extension-examples: no such file or directory
[start-services] ERROR: docker compose up failed
```
- The error is about the context path, not the Dockerfile. Misleading when the real problem is the Dockerfile.

### Test B: Fixed context, wrong Dockerfile
```
#2 [types-server internal] load build definition from Dockerfile.WRONG
#2 transferring dockerfile: 2B done
#2 DONE 0.0s
target types-server: failed to solve: failed to read dockerfile: open Dockerfile.WRONG: no such file or directory

[start-services] ERROR: docker compose up failed
```
- Docker's error is clear: `failed to read dockerfile: open Dockerfile.WRONG: no such file or directory`
- The script's error is generic: just `docker compose up failed` — no indication of *why*.

## Analysis

**Two findings:**

1. **Worktree/context interaction (INTERESTING):** The `context: ../..` in docker-compose.yaml uses a relative path that only works when the file is at its expected location in the directory tree. Running from a git worktree in a nested location silently changes the context resolution, producing a confusing error. This isn't a bug per se — it's an inherent limitation of relative Docker build contexts — but it means **the chaos worktree setup cannot directly run Docker Compose builds without patching context paths**. This is a property of the testing setup, not the deployment scripts themselves.

2. **Script error message (MINOR):** When docker compose fails, `start-services.sh` only reports `docker compose up failed`. It doesn't capture or surface the Docker build error. Docker's own error (`failed to read dockerfile`) is clear, but it scrolls by in the build output — the final script error doesn't include it. Consider capturing the Docker error and including it in the die message.

## Ideas for Next Time
- Test scenario #13 (wrong-ports) — change EXTENSION_PORT and see if test.sh explains the mismatch clearly
- Test scenario #11 (hash-mismatch) — modify OPType constants and rebuild; requires working Docker build
- Investigate whether start-services.sh could capture the docker compose stderr and include it in the error message
- Try scenario #10 (skip-preflight) — this doesn't need Docker builds, just pre-build.sh with chain access
