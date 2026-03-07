import dumpJson from "./extracted_dump.json" assert { type: "json" };

export function initializeHooks(): Promise<void> {
  return new Promise((resolve, reject) => {
    const tryInit = () => {
      globalThis.libil2cpp = Process.getModuleByName("libil2cpp.so");
      console.log("[*] libil2cpp.so loaded at:", globalThis.libil2cpp.base);

      globalThis.dump = dumpJson;
      console.log("[*] Dump loaded");
      resolve();
    };

    try {
      tryInit();
    } catch (e) {
      reject(e);
    }
  });
}

export function hook(
  name: string,
  offset: number,
  callbacks: InvocationListenerCallbacks
) {
  const ptr = globalThis.libil2cpp.base.add(offset);
  Interceptor.attach(ptr, callbacks);
  console.log(`[*] Hook ${name}`);
}

export function getMethodFromDump(name: string) {
  const method = globalThis.dump[name];
  if (!method) {
    throw new Error(`Method ${name} not found`);
  }
  return method;
}

export function hookDumpMethod(
  name: string,
  callbacks: InvocationListenerCallbacks
) {
  const { Name, Address } = getMethodFromDump(name);
  console.log(`[*] Hooking ${Name} at 0x${Address.toString(16)}`);
  hook(Name, Address, callbacks);
}

export function readStr(addr: NativePointer) {
  if (addr.isNull()) return "<null>";
  try {
    return addr.add(0x14).readUtf16String();
  } catch (e) {
    return "<err>";
  }
}

export function writeStr(addr: NativePointer, text: string) {
  addr.add(0x10).writeInt(text.length);
  addr.add(0x14).writeUtf16String(text);
}

export function patchStringLiteral(cacheSlotOffset: number, newValue: string) {
  const il2cpp_string_new = new NativeFunction(
    globalThis.libil2cpp.findExportByName("il2cpp_string_new")!,
    "pointer",
    ["pointer"]
  );

  const slot = globalThis.libil2cpp.base.add(cacheSlotOffset);
  const newStr = il2cpp_string_new(Memory.allocUtf8String(newValue));

  Memory.protect(slot, Process.pointerSize, "rw-");
  slot.writePointer(newStr);
}

export function scanAndPatchIl2cppString(
  searchStr: string,
  replaceStr: string
): number {
  // Build UTF-16LE byte pattern
  const bytes: string[] = [];
  for (const ch of searchStr) {
    const code = ch.charCodeAt(0);
    bytes.push((code & 0xff).toString(16).padStart(2, "0"));
    bytes.push(((code >> 8) & 0xff).toString(16).padStart(2, "0"));
  }
  const pattern = bytes.join(" ");

  let patched = 0;
  for (const range of Process.enumerateRanges("r--")) {
    try {
      const matches = Memory.scanSync(range.base, range.size, pattern);
      for (const m of matches) {
        const lengthPtr = m.address.sub(4);
        const len = lengthPtr.readS32();
        if (len === searchStr.length) {
          Memory.protect(m.address.sub(4), replaceStr.length * 2 + 6, "rw-");
          lengthPtr.writeS32(replaceStr.length);
          m.address.writeUtf16String(replaceStr);
          patched++;
        }
      }
    } catch (e) {
      /* skip */
    }
  }

  return patched;
}
