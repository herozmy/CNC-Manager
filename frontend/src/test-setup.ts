import '@testing-library/jest-dom/vitest'

// jsdom does not implement the native modal lifecycle. Browser checks cover focus and inertness.
if (!HTMLDialogElement.prototype.showModal) {
  HTMLDialogElement.prototype.showModal = function () {
    this.open = true
    this.querySelector<HTMLElement>('button')?.focus()
  }
  HTMLDialogElement.prototype.close = function () { this.open = false }
}
