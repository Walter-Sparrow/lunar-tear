import { readFileSync, writeFileSync } from "fs";
import path from "path";
import { OriginalDump, ExtractedDump } from "./types";
import { dumpMethods } from "./dump_methods";

const DUMP_PATH = path.join(__dirname, "../../client/dump_output/script.json");
const OUTPUT_PATH = path.join(__dirname, "./extracted_dump.json");

const dump = JSON.parse(readFileSync(DUMP_PATH, "utf8")) as OriginalDump;

export function getAllMethodNames(obj: Record<string, any>): string[] {
  const result: string[] = [];

  function recurse(value: any) {
    if (typeof value === "string") {
      result.push(value);
      return;
    }
    if (typeof value === "object" && value !== null) {
      Object.values(value).forEach(recurse);
    }
  }

  recurse(obj);
  return result;
}

const methods: ExtractedDump = {};

for (const method of getAllMethodNames(dumpMethods)) {
  const methodData = dump.ScriptMethod.find((m) => m.Name.endsWith(method));
  if (methodData) {
    methods[method] = methodData;
  } else {
    console.warn(`Method ${method} not found in dump`);
  }
}

writeFileSync(OUTPUT_PATH, JSON.stringify(methods, null, 2));
console.log(`Saved extracted dump to ${OUTPUT_PATH}`);
