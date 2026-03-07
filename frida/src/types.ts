export type DumpMethod = {
  Name: string;
  Address: number;
  Signature: string;
  TypeSignature: string;
};

export type DumpString = {
  Address: number;
  Value: string;
};

export type OriginalDump = {
  ScriptMethod: DumpMethod[];
  ScriptString: DumpString[];
};

export type ExtractedDump = {
  [key: string]: DumpMethod;
};
