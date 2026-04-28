#!/usr/bin/env node

/**
 * Copies skills/ into .claude/skills/ so they're available locally
 * as slash commands during development.
 *
 * Patches `npx pr-wrangler-reviews` to `node <repo>/bin/pr-wrangler-reviews.js`
 * so the local dev version is used instead of the published npm package.
 *
 * Run: node scripts/install-skills.js
 */

const fs = require("node:fs");
const path = require("node:path");

const ROOT = path.resolve(__dirname, "..");
const SRC = path.join(ROOT, "skills");
const DEST = path.join(ROOT, ".claude", "skills");
const LOCAL_CLI = path.join(ROOT, "bin", "pr-wrangler-reviews.js");
const shellQuote = (value) => `"${String(value).replace(/(["\\$`])/g, "\\$1")}"`;
const LOCAL_CLI_CMD = `node ${shellQuote(LOCAL_CLI)}`;

const SKILL_DIRS = ["pr-wrangler-review-comments", "pr-wrangler-review-bot-comments", "pr-wrangler-review-human-comments"];

fs.mkdirSync(DEST, { recursive: true });

for (const name of SKILL_DIRS) {
  const skillDest = path.join(DEST, name);
  fs.mkdirSync(skillDest, { recursive: true });

  let content = fs.readFileSync(path.join(SRC, name, "SKILL.md"), "utf8");

  // Patch all package runner references to use local CLI binary
  content = content.replaceAll("npx pr-wrangler-reviews", LOCAL_CLI_CMD);
  content = content.replaceAll("pnpm dlx pr-wrangler-reviews", LOCAL_CLI_CMD);
  content = content.replaceAll("yarn dlx pr-wrangler-reviews", LOCAL_CLI_CMD);
  content = content.replaceAll("bunx pr-wrangler-reviews", LOCAL_CLI_CMD);

  // Deduplicate allowed-tools after patching (all runners become the same)
  content = content.replace(
    /^(allowed-tools:).*$/m,
    `$1 Bash(${LOCAL_CLI_CMD} *) Bash(git config *) Bash(git add *) Bash(git commit *) Bash(git push *)`
  );

  // Remove the package manager substitution note (irrelevant in local dev)
  content = content.replace(
    /All commands below use [^\n]*\. If the project uses a different package manager[^\n]*\. Honor the user's package manager preference throughout\.\n\n/,
    ""
  );

  fs.writeFileSync(path.join(skillDest, "SKILL.md"), content);
  console.log(`Installed ${name} -> .claude/skills/${name}/ (patched for local dev)`);
}
