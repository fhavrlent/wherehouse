/** Names of apps that link to things, for showing their links. */
export const APP_NAMES: Record<string, string> = { warranteed: "Warranteed" };

export const appName = (source: string) => APP_NAMES[source] ?? source;
