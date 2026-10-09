// @vitest-environment node
import { describe, expect, it } from "vitest";
import { shouldFocusAfterTrigger } from "./focus-trigger";

describe("composer expand trigger focus", () => {
  it("waits for native input layout when the first trigger expands the composer", () => {
    expect(shouldFocusAfterTrigger(false)).toBe(true);
  });

  it("requests focus immediately for another starter selection while expanded", () => {
    expect(shouldFocusAfterTrigger(true)).toBe(false);
  });

  it("requests focus immediately when the reply target changes while expanded", () => {
    expect(shouldFocusAfterTrigger(true)).toBe(false);
  });
});
