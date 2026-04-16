/**
 * Platform abstraction — macOS only.
 */

let _platformPromise = null;

export function getPlatform() {
  if (!_platformPromise) {
    if (process.platform !== "darwin") {
      _platformPromise = Promise.reject(
        new Error("port-whisperer requires macOS"),
      );
    } else {
      _platformPromise = import("./darwin.js");
    }
  }
  return _platformPromise;
}
