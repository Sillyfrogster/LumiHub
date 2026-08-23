import { readFile, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { bundle, createConfig, stringifyYaml } from "@redocly/openapi-core";

const source = fileURLToPath(
  new URL("../../api/openapi/openapi.yaml", import.meta.url),
);
const output = fileURLToPath(
  new URL("../../api/openapi/openapi.gen.yaml", import.meta.url),
);
const config = await createConfig({});
const result = await bundle({ ref: source, config });
const errors = result.problems.filter(({ severity }) => severity === "error");

if (errors.length > 0) {
  for (const problem of errors) {
    const location = problem.location?.[0];
    const position = location?.start
      ? `:${location.start.line}:${location.start.col}`
      : "";
    console.error(
      `${location?.source?.absoluteRef ?? source}${position} ${problem.message}`,
    );
  }
  process.exit(1);
}

const notice =
  "# Generated from the OpenAPI modules. Run make generate after editing them.\n";
const contract = notice + stringifyYaml(result.bundle.parsed);

if (process.argv.includes("--check")) {
  const published = await readFile(output, "utf8").catch(() => "");
  if (published !== contract) {
    console.error("The OpenAPI bundle is stale. Run make generate.");
    process.exit(1);
  }
} else {
  await writeFile(output, contract);
}
