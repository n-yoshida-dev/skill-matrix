// E2E で差し替え前・後のサイトを配信するポート。playwright.config.ts とテストの両方から使う
export const E2E_PORTS = { before: 4181, after: 4182 } as const
