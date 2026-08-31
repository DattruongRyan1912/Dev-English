import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

const script = resolve(dirname(fileURLToPath(import.meta.url)), "backend_coverage_gate.mjs");

function runGate(directory, profile, ...args) {
  const profilePath = join(directory, "coverage.out");
  writeFileSync(profilePath, `${profile.trim()}\n`);
  try {
    const output = execFileSync(process.execPath, [script, "--profile", profilePath, ...args], {
      encoding: "utf8",
      stdio: ["ignore", "pipe", "pipe"],
    });
    return { status: 0, output };
  } catch (error) {
    return {
      status: error.status ?? -1,
      output: `${error.stdout ?? ""}${error.stderr ?? ""}`,
    };
  }
}

function withTempDirectory(callback) {
  const directory = mkdtempSync(join(tmpdir(), "devenglish-coverage-gate-"));
  try {
    return callback(directory);
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
}

test("accepts the exact raw threshold and an equal-or-lower baseline", () => {
  withTempDirectory((directory) => {
    const baselinePath = join(directory, "baseline.json");
    writeFileSync(
      baselinePath,
      JSON.stringify({ coveredStatements: 8, totalStatements: 10, baselineRef: "test" }),
    );
    const result = runGate(
      directory,
      [
        "mode: set",
        "backend/internal/application/service.go:1.1,2.1 9 1",
        "backend/internal/application/validation.go:1.1,2.1 1 0",
      ].join("\n"),
      "--package",
      "internal/application",
      "--min",
      "90",
      "--baseline-manifest",
      baselinePath,
    );

    assert.equal(result.status, 0, result.output);
    assert.match(result.output, /internal\/application\s+90\.00% \(9\/10\)\s+PASS/);
    assert.match(result.output, /R0 baseline floor: 80\.00% \(8\/10\)\s+\[PASS; test\]/);
    assert.match(result.output, /GATE: PASS/);
  });
});

test("rejects a rounded-looking package that is below the raw threshold", () => {
  withTempDirectory((directory) => {
    const result = runGate(
      directory,
      [
        "mode: set",
        "backend/internal/application/service.go:1.1,2.1 899 1",
        "backend/internal/application/validation.go:1.1,2.1 100 0",
      ].join("\n"),
      "--package",
      "internal/application",
      "--min",
      "90",
    );

    assert.equal(result.status, 1, result.output);
    assert.match(result.output, /89\.99% \(899\/999\)\s+FAIL/);
    assert.match(result.output, /GATE: FAIL/);
  });
});

test("rejects a current total below the recorded baseline floor", () => {
  withTempDirectory((directory) => {
    const baselinePath = join(directory, "baseline.json");
    writeFileSync(
      baselinePath,
      JSON.stringify({ coveredStatements: 10, totalStatements: 10, baselineRef: "test" }),
    );
    const result = runGate(
      directory,
      [
        "mode: set",
        "backend/internal/application/service.go:1.1,2.1 9 1",
        "backend/internal/application/validation.go:1.1,2.1 1 0",
      ].join("\n"),
      "--package",
      "internal/application",
      "--min",
      "90",
      "--baseline-manifest",
      baselinePath,
    );

    assert.equal(result.status, 1, result.output);
    assert.match(result.output, /R0 baseline floor: 100\.00% \(10\/10\)\s+\[FAIL; test\]/);
    assert.match(result.output, /overall coverage is below the recorded R0 floor/);
  });
});

test("fails closed for malformed profile and baseline input", () => {
  withTempDirectory((directory) => {
    const malformedProfile = runGate(directory, "mode: set\nnot a Go coverprofile record");
    assert.equal(malformedProfile.status, 2, malformedProfile.output);
    assert.match(malformedProfile.output, /invalid Go coverprofile record/);

    const baselinePath = join(directory, "baseline.json");
    writeFileSync(baselinePath, JSON.stringify({ coveredStatements: 11, totalStatements: 10 }));
    const malformedBaseline = runGate(
      directory,
      [
        "mode: set",
        "backend/internal/application/service.go:1.1,2.1 10 10",
      ].join("\n"),
      "--package",
      "internal/application",
      "--baseline-manifest",
      baselinePath,
    );
    assert.equal(malformedBaseline.status, 2, malformedBaseline.output);
    assert.match(malformedBaseline.output, /must contain safe integer coveredStatements/);
  });
});
