import { buildPackage } from "../scripts/build.js"

export default async function buildBeforeTests() {
  await buildPackage()
}
