import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../../lib/api";
import type { VisualFolder, VisualFolders } from "../../lib/types";

export type FolderKind = "containers" | "vms";

const EMPTY: VisualFolders = { version: 1, folders: [], assignments: {} };

export function newFolderID(): string {
  return `f-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 6)}`;
}

export function useVisualFolders(kind: FolderKind) {
  const queryClient = useQueryClient();
  const queryKey = ["visual-folders", kind];
  const query = useQuery({
    queryKey,
    queryFn: () => (kind === "containers" ? api.containerFolders() : api.vmFolders()),
  });
  const data = query.data ?? EMPTY;
  const save = useMutation({
    mutationFn: (layout: VisualFolders) =>
      kind === "containers" ? api.saveContainerFolders(layout) : api.saveVMFolders(layout),
    onSuccess: (saved) => queryClient.setQueryData(queryKey, saved),
    onError: () => {
      void queryClient.invalidateQueries({ queryKey });
    },
  });

  const commit = (layout: VisualFolders) => {
    queryClient.setQueryData(queryKey, layout);
    save.mutate(layout);
  };

  const assignment = (name: string): string => {
    const folder = data.assignments[name] ?? "";
    return data.folders.some((item) => item.id === folder) ? folder : "";
  };

  const setAssignment = (name: string, folderID: string) => {
    const assignments = { ...data.assignments };
    if (folderID && data.folders.some((item) => item.id === folderID)) assignments[name] = folderID;
    else delete assignments[name];
    commit({ ...data, assignments });
  };

  const createFolder = (name: string): VisualFolder | null => {
    const trimmed = name.trim();
    if (!trimmed) return null;
    const existing = data.folders.find((folder) => folder.name.toLowerCase() === trimmed.toLowerCase());
    if (existing) return existing;
    const folder: VisualFolder = { id: newFolderID(), name: trimmed, order: data.folders.length };
    commit({ ...data, folders: [...data.folders, folder] });
    return folder;
  };

  const renameFolder = (id: string, name: string): boolean => {
    const trimmed = name.trim();
    if (!trimmed) return false;
    if (data.folders.some((folder) => folder.id !== id && folder.name.toLowerCase() === trimmed.toLowerCase())) {
      return false;
    }
    commit({
      ...data,
      folders: data.folders.map((folder) => (folder.id === id ? { ...folder, name: trimmed } : folder)),
    });
    return true;
  };

  const deleteFolder = (id: string) => {
    const assignments = Object.fromEntries(
      Object.entries(data.assignments).filter(([, folderID]) => folderID !== id),
    );
    commit({
      ...data,
      folders: data.folders.filter((folder) => folder.id !== id),
      assignments,
    });
  };

  const moveFolder = (id: string, delta: number) => {
    const folders = [...data.folders];
    const index = folders.findIndex((folder) => folder.id === id);
    const target = index + delta;
    if (index < 0 || target < 0 || target >= folders.length) return;
    [folders[index], folders[target]] = [folders[target], folders[index]];
    commit({ ...data, folders: folders.map((folder, order) => ({ ...folder, order })) });
  };

  const saveFolders = (folders: VisualFolder[], assignments: Record<string, string>) => {
    commit({ version: 1, folders, assignments });
  };

  return {
    ...data,
    loading: query.isLoading,
    saving: save.isPending,
    assignment,
    setAssignment,
    createFolder,
    renameFolder,
    deleteFolder,
    moveFolder,
    saveFolders,
  };
}

export type VisualFolderState = ReturnType<typeof useVisualFolders>;
