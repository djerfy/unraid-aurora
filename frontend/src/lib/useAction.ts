import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../lib/api";
import type { ActionKind } from "../lib/types";
import { useToast } from "../app/ToastProvider";

type ActionInput = {
  kind: ActionKind;
  target?: string;
  success: string;
  description?: string;
};

/** Runs a whitelisted server action and refreshes the snapshot on success. */
export function useAction() {
  const queryClient = useQueryClient();
  const { toast } = useToast();

  return useMutation({
    mutationFn: ({ kind, target = "" }: ActionInput) => api.action(kind, target),
    onSuccess: (_data, variables) => {
      toast({ kind: "success", title: variables.success, description: variables.description });
      void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
    },
    onError: (error: Error, variables) => {
      toast({ kind: "error", title: "Action failed", description: error.message });
      void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
      console.error("action failed", variables.kind, variables.target, error);
    },
  });
}
