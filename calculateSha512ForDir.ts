import { walk } from "https://deno.land/std@0.222.0/fs/mod.ts";
import { createHash } from "https://deno.land/std@0.157.0/hash/mod.ts";
import { encode } from "https://deno.land/std@0.136.0/encoding/base64.ts";
import { fromFileUrl } from "https://deno.land/std@0.222.0/path/from_file_url.ts";
async function calculateSha512ForDir(
  dirPath: string,
): Promise<Record<string, string>> {
  const fileHashes: Record<string, string> = {};

  for await (const entry of walk(dirPath)) {
    console.log(entry);
    if (entry.isFile) {
      const filePath = entry.path;
      const fileContent = await Deno.readFile(filePath);
      const hash = createHash("sha512");
      hash.update(fileContent);
      const sha512Base64 = encode(hash.digest());
      fileHashes[
        filePath.replaceAll(dirPath, "").replaceAll("\\", "/")
      ] = sha512Base64;
    }
  }

  return fileHashes;
}
if (import.meta.main) {
  // 使用示例
  await (async () => {
    const dirPath = fromFileUrl(new URL(import.meta.resolve("./static/dist")));
    console.log(dirPath);
    const fileHashJson = await calculateSha512ForDir(dirPath);
    console.log(JSON.stringify(fileHashJson, null, 2));
    await Deno.writeTextFile(
      "./static/dist.json",
      JSON.stringify(fileHashJson, null, 2),
    );
  })();
}
