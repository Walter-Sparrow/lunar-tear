import {
  hookDumpMethod,
  initializeHooks,
  patchStringLiteral,
  readStr,
  scanAndPatchIl2cppString,
  writeStr,
} from "./utils";
import { dumpMethods } from "./dump_methods";

// [NetworkConfig] api.app.nierreincarnation.com

const SERVER_ADDRESS = "127.0.0.1";
const HTTP_PORT = 8080;
const GRPC_PORT = 7777;

const webUrl = `http://${SERVER_ADDRESS}:${HTTP_PORT}`;

async function main() {
  try {
    await initializeHooks();
  } catch (e) {
    console.error("[!] Error initializing hooks:", e);
    return;
  }

  let urlsPatched = false;
  const patches = [
    { search: "https://web.app.nierreincarnation.com", replace: webUrl },
    {
      search: "https://resources-api.app.nierreincarnation.com/",
      replace: `${webUrl}/`,
    },
    {
      search:
        "https://web.app.nierreincarnation.com/assets/release/{0}/database.bin",
      replace: `${webUrl}/assets/release/{0}/database.bin`,
    },
  ];

  hookDumpMethod(dumpMethods.NetworkConfig.GetServerAddress, {
    onEnter() {
      if (!urlsPatched) {
        urlsPatched = true;
        for (const { search, replace } of patches) {
          const count = scanAndPatchIl2cppString(search, replace);
          console.log(`[*] "${search}" -> "${replace}" (${count} patched)`);
        }
      }
    },
    onLeave(retval) {
      for (const { search, replace } of patches) {
        const count = scanAndPatchIl2cppString(search, replace);
        console.log(`[*] "${search}" -> "${replace}" (${count} patched)`);
      }
      writeStr(retval, `${SERVER_ADDRESS}:${GRPC_PORT}`);
    },
  });
}

main();
