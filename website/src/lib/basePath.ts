export function buildBasePath(repository: string | undefined, githubActions: boolean): string {
  const name = repository?.split("/")[1] ?? "IncidentGPT";
  return githubActions ? `/${name}/` : "/";
}
