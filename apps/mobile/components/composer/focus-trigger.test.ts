// @vitest-environment node
import { describe, expect, it } from "vitest";
import { shouldWaitForInputLayout } from "./focus-trigger";

describe("composer expand trigger focus", () => {
  it("waits for native input layout when the first trigger expands the composer", () => {
    expect(shouldWaitForInputLayout(false)).toBe(true);
  });

  it("requests focus immediately for another starter selection while expanded", () => {
    expect(shouldWaitForInputLayout(true)).toBe(false);
  });

  it("requests focus immediately when the reply target changes while expanded", () => {
    expect(shouldWaitForInputLayout(true)).toBe(false);
  });
});
