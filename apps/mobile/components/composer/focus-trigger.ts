/** Whether an expand trigger must wait for the TextInput's first native layout. */
export function shouldWaitForInputLayout(isExpanded: boolean): boolean {
  return !isExpanded;
}
