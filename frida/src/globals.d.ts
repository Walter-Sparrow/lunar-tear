import { ExtractedDump } from "./types";

declare global {
  namespace globalThis {
    var libil2cpp: Module;
    var dump: ExtractedDump;
  }
}
