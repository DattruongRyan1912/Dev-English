#!/usr/bin/env node

// Enforce the DevEnglish backend package coverage contract from a Go coverprofile.
// This is intentionally independent from CyberOS's read-only coverage-scope
// skeleton: it judges raw statement counts and never treats rounded percentages
// as proof. The profile must come from the same checkout and test environment
// that is being gated.

import { readFileSync } from "node:fs";
import { basename, resolve } from "node:path";

const DEFAULT_PACKAGES = [
  "internal/application",
  "internal/assistant",
  "internal/connectors",
  "internal/httpapi",
  "internal/knowledge",
  "internal/mcp",
  "internal/work",
];

class UsageError extends Error {}
class ProfileError extends Error {}

function usage() {
  return `Usage:
  node scripts/backend_coverage_gate.mjs --profile <coverprofile> [options]

Options:
  --package <path>  Package relative to backend/; may be repeated.
                    Defaults to the seven V1 core packages.
  --min <integer>   Minimum statement coverage percentage (default: 90).
  --baseline-manifest <json>
                    Compare the overall raw ratio with a recorded baseline.
  --help            Show this help.

Exit codes:
  0  all required packages and the optional overall floor pass
  1  a required package or the optional overall floor fails
  2  invalid arguments or invalid profile
`;
}

function parseArgs(argv) {
  const options = { packages: [], min: 90, profile: null, baselineManifest: null };
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === "--help" || arg === "-h") {
      console.log(usage());
      process.exit(0);
    }
    if (arg === "--profile") {
      options.profile = argv[++i];
      continue;
    }
    if (arg === "--package") {
      options.packages.push(argv[++i]);
      continue;
    }
    if (arg === "--min") {
      const value = argv[++i];
      if (!/^\d+$/.test(value || "")) {
        throw new UsageError("--min must be an integer percentage");
      }
      options.min = Number(value);
      continue;
    }
    if (arg === "--baseline-manifest") {
      options.baselineManifest = argv[++i];
      continue;
    }
    throw new UsageError(`unknown argument: ${arg}`);
  }

  if (!options.profile) throw new UsageError("--profile is required");
  if (options.min < 0 || options.min > 100) {
    throw new UsageError("--min must be between 0 and 100");
  }
  options.packages = options.packages.length > 0 ? options.packages : DEFAULT_PACKAGES;
  if (options.packages.some((name) => !/^([a-z0-9_.-]+\/)*[a-z0-9_.-]+$/.test(name))) {
    throw new UsageError("--package must be a normalized backend-relative path");
  }
  if (new Set(options.packages).size !== options.packages.length) {
    throw new UsageError("--package values must be unique");
  }
  return options;
}

function backendRelativePath(profilePath) {
  const normalized = profilePath.replaceAll("\\", "/");
  const marker = "/backend/";
  const markerIndex = normalized.lastIndexOf(marker);
  if (markerIndex >= 0) return normalized.slice(markerIndex + marker.length);
  if (normalized.startsWith("backend/")) return normalized.slice("backend/".length);
  return null;
}

function packageForProfilePath(profilePath) {
  const relativePath = backendRelativePath(profilePath);
  if (!relativePath) return null;
  const slash = relativePath.lastIndexOf("/");
  return slash >= 0 ? relativePath.slice(0, slash) : null;
}

function parseProfile(profileText) {
  const lines = profileText.split(/\r?\n/);
  const header = lines.shift();
  if (header !== "mode: set" && header !== "mode: count" && header !== "mode: atomic") {
    throw new ProfileError("profile must begin with a supported Go coverage mode");
  }

  const overall = { covered: 0, total: 0 };
  const packages = new Map();
  let records = 0;
  for (const line of lines) {
    if (line.trim() === "") continue;
    const match = /^(.*):(\d+)\.\d+,(\d+)\.\d+\s+(\d+)\s+(\d+)$/.exec(line);
    if (!match) throw new ProfileError(`invalid Go coverprofile record: ${line}`);
    const [, file, , , statementCountText, hitCountText] = match;
    const statementCount = Number(statementCountText);
    const hitCount = Number(hitCountText);
    if (!Number.isSafeInteger(statementCount) || statementCount < 0) {
      throw new ProfileError(`invalid statement count in record: ${line}`);
    }
    if (!Number.isSafeInteger(hitCount) || hitCount < 0) {
      throw new ProfileError(`invalid hit count in record: ${line}`);
    }

    overall.total += statementCount;
    if (hitCount > 0) overall.covered += statementCount;
    const packageName = packageForProfilePath(file);
    if (!packageName) {
      records += 1;
      continue;
    }
    const current = packages.get(packageName) || { covered: 0, total: 0 };
    current.total += statementCount;
    if (hitCount > 0) current.covered += statementCount;
    packages.set(packageName, current);
    records += 1;
  }
  if (records === 0 || overall.total === 0) {
    throw new ProfileError("coverage profile contains no statement records");
  }
  return { overall, packages, records };
}

function percent(metric) {
  return metric.total === 0 ? 100 : (metric.covered / metric.total) * 100;
}

function meetsThreshold(metric, minimum) {
  // Keep this comparison integral. A displayed 90.00% can still be below 90
  // when the raw numerator/denominator are considered.
  return metric.total > 0 && metric.covered * 100 >= metric.total * minimum;
}

function displayMetric(metric) {
  return `${percent(metric).toFixed(2)}% (${metric.covered}/${metric.total})`;
}

function readBaselineManifest(path) {
  let value;
  try {
    value = JSON.parse(readFileSync(resolve(path), "utf8"));
  } catch (error) {
    throw new ProfileError(`cannot read baseline manifest '${path}': ${error.message}`);
  }
  const covered = value?.coveredStatements;
  const total = value?.totalStatements;
  if (!Number.isSafeInteger(covered) || covered < 0 ||
      !Number.isSafeInteger(total) || total <= 0 || covered > total) {
    throw new ProfileError(
      `baseline manifest '${path}' must contain safe integer coveredStatements and totalStatements`,
    );
  }
  return { covered, total, ref: value.baselineRef || "unknown" };
}

function run(options) {
  let profileText;
  try {
    profileText = readFileSync(resolve(options.profile), "utf8");
  } catch (error) {
    throw new ProfileError(`cannot read profile '${options.profile}': ${error.message}`);
  }
  const parsed = parseProfile(profileText);
  const baseline = options.baselineManifest
    ? readBaselineManifest(options.baselineManifest)
    : null;
  const results = options.packages.map((name) => {
    const metric = parsed.packages.get(name);
    return {
      name,
      metric,
      status: metric && meetsThreshold(metric, options.min) ? "PASS" : "FAIL",
    };
  });
  const failed = results.filter((result) => result.status !== "PASS");
  const baselinePass = !baseline ||
    parsed.overall.covered * baseline.total >= baseline.covered * parsed.overall.total;

  console.log(`Backend coverage gate (${basename(options.profile)})`);
  console.log(`Overall: ${displayMetric(parsed.overall)}`);
  if (baseline) {
    console.log(
      `R0 baseline floor: ${displayMetric({ covered: baseline.covered, total: baseline.total })}` +
      ` [${baselinePass ? "PASS" : "FAIL"}; ${baseline.ref}]`,
    );
  }
  console.log(`Required raw statement threshold: >= ${options.min}%`);
  console.log("");
  console.log("Package                         Coverage             Status");
  console.log("--------------------------------------------------------------");
  for (const result of results) {
    const coverage = result.metric ? displayMetric(result.metric) : "missing from profile";
    console.log(`${result.name.padEnd(31)} ${coverage.padEnd(21)} ${result.status}`);
  }
  console.log("");
  if (failed.length > 0 || !baselinePass) {
    const reasons = [];
    if (failed.length > 0) reasons.push(`${failed.length} package(s) below threshold or missing`);
    if (!baselinePass) reasons.push("overall coverage is below the recorded R0 floor");
    console.log(`GATE: FAIL (${reasons.join("; ")})`);
    return 1;
  }
  console.log("GATE: PASS");
  return 0;
}

try {
  const options = parseArgs(process.argv.slice(2));
  process.exitCode = run(options);
} catch (error) {
  const code = error instanceof UsageError || error instanceof ProfileError ? 2 : 1;
  console.error(`${error.name}: ${error.message}`);
  if (error instanceof UsageError) console.error(usage());
  process.exitCode = code;
}
