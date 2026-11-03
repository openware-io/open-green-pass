/**
 * Cross-project URL restore guards.
 *
 * A deep link can target a project that differs from the project the object
 * catalog currently has loaded. Until the catalog has fully loaded that
 * project (project id + repositories + services), the draft must NOT be
 * validated or defaulted against the stale catalog: doing so would silently
 * drop the URL's service ids and would read repository refs with the wrong
 * project id.
 */
export function shouldValidateDraftProject(args: {
  projectsLoaded: boolean;
  catalogLoading: boolean;
  draftProjectId: number;
  catalogProjectId: number;
  pendingProjectSync: number | null;
}): boolean {
  const { projectsLoaded, catalogLoading, draftProjectId, catalogProjectId, pendingProjectSync } = args;
  if (!projectsLoaded || catalogLoading) return false;
  if (pendingProjectSync !== null) return false;
  if (draftProjectId <= 0 || catalogProjectId <= 0) return false;
  return draftProjectId === catalogProjectId;
}

/** True when catalog data (repos/services) belongs to the draft's project. */
export function draftMatchesCatalogData(draftProjectId: number, catalogProjectId: number, catalogLoading: boolean): boolean {
  if (catalogLoading) return false;
  if (draftProjectId <= 0 || catalogProjectId <= 0) return false;
  return draftProjectId === catalogProjectId;
}
