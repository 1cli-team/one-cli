export function fail(en: string, zh: string): never {
  throw new Error(`${en}\n${zh}`);
}

export function say(en: string, zh: string) {
  console.log(`${en}\n${zh}`);
}
