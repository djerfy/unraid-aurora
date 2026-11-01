import { useMemo, useState, type ReactNode } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Check,
  Clock3,
  FolderTree,
  KeyRound,
  Lock,
  RefreshCw,
  Minus,
  Search,
  ShieldCheck,
  Plus,
  Trash2,
  UserCog,
  Users,
} from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { EntityIcon, userIconURL } from "../components/ui/entity-icon";
import { Drawer, Menu, Modal } from "../components/ui/overlays";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  IconButton,
  InputBase,
} from "../components/ui/primitives";
import { useApp } from "../app/AppDataProvider";
import { useToast } from "../app/ToastProvider";
import { api } from "../lib/api";
import { formatDateTime, relativeTime } from "../lib/format";

export function UsersPage() {
  const { snapshot, refresh } = useApp();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const [query, setQuery] = useState("");
  const [selectedName, setSelectedName] = useState<string | null>(null);
  const [addOpen, setAddOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [passwordOpen, setPasswordOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [addDraft, setAddDraft] = useState({ name: "", description: "", password: "", confirm: "" });
  const [description, setDescription] = useState("");
  const [password, setPassword] = useState("");
  const [passwordConfirm, setPasswordConfirm] = useState("");
  const [error, setError] = useState("");

  const invalidateUsers = () => {
    void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
  };
  const createUser = useMutation({
    mutationFn: api.createUser,
    onSuccess: () => {
      toast({ kind: "success", title: "User created", description: addDraft.name });
      setAddOpen(false);
      invalidateUsers();
    },
    onError: (mutationError: Error) => setError(mutationError.message),
  });
  const updateUser = useMutation({
    mutationFn: ({ name, value }: { name: string; value: string }) => api.updateUser(name, value),
    onSuccess: () => {
      toast({ kind: "success", title: "User updated" });
      setEditOpen(false);
      invalidateUsers();
    },
    onError: (mutationError: Error) => setError(mutationError.message),
  });
  const changePassword = useMutation({
    mutationFn: ({ name, value }: { name: string; value: string }) => api.changeUserPassword(name, value),
    onSuccess: () => {
      toast({ kind: "success", title: "Password changed" });
      setPasswordOpen(false);
      setPassword("");
      setPasswordConfirm("");
    },
    onError: (mutationError: Error) => setError(mutationError.message),
  });
  const deleteUser = useMutation({
    mutationFn: (name: string) => api.deleteUser(name),
    onSuccess: () => {
      toast({ kind: "info", title: "User deleted" });
      setDeleteOpen(false);
      setSelectedName(null);
      invalidateUsers();
    },
    onError: (mutationError: Error) => setError(mutationError.message),
  });

  if (!snapshot) return null;
  const users = snapshot.users;
  const selected = users.find((user) => user.name === selectedName) ?? null;
  const admins = users.filter((user) => user.role === "administrator");
  const readOnly = users.filter((user) => user.role === "read-only");
  const shares = new Set(users.flatMap((user) => user.shares));

  const filtered = useMemo(
    () =>
      users.filter((user) => {
        if (!query.trim()) return true;
        const haystack = `${user.name} ${user.description} ${user.role}`.toLowerCase();
        return haystack.includes(query.toLowerCase());
      }),
    [users, query],
  );

  return (
    <>
      <PageHeader
        eyebrow="System"
        title="Users"
        description="Accounts with access to the web interface, SMB shares and NFS exports."
        meta={
          <>
            <Badge tone="accent" dot>{users.length} accounts</Badge>
            <span className="chip">{admins.length} administrators</span>
          </>
        }
        actions={
          <>
            <Button variant="secondary" icon={<RefreshCw className="size-3.5" />} onClick={refresh}>
              Refresh
            </Button>
            <Button
              variant="primary"
              icon={<Plus className="size-3.5" />}
              onClick={() => {
                setAddDraft({ name: "", description: "", password: "", confirm: "" });
                setError("");
                setAddOpen(true);
              }}
            >
              Add user
            </Button>
          </>
        }
      />

      <div className="mb-5 grid grid-cols-2 gap-3.5 lg:grid-cols-4">
        <Stat icon={<Users className="size-4" />} label="Accounts" value={String(users.length)} hint="local users" />
        <Stat icon={<ShieldCheck className="size-4" />} label="Administrators" value={String(admins.length)} hint="full web UI access" />
        <Stat icon={<Lock className="size-4" />} label="Read-only" value={String(readOnly.length)} hint="restricted shares" />
        <Stat icon={<FolderTree className="size-4" />} label="Scoped shares" value={String(shares.size)} hint="across all accounts" />
      </div>

      <Card>
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-line px-5 py-3.5">
          <div>
            <h2 className="text-[13px] font-semibold text-ink">Local accounts</h2>
            <p className="mt-0.5 text-[10.5px] text-subtle">Managed by Unraid · permissions apply to SMB and NFS</p>
          </div>
          <InputBase
            value={query}
            onChange={setQuery}
            placeholder="Search users…"
            icon={<Search className="size-3.5" />}
            className="w-52"
          />
        </div>

        {filtered.length === 0 ? (
          <EmptyState icon={<Users className="size-5" />} title="No users match your search" description="Try another name or clear the filter." />
        ) : (
          <>
            <div className="hidden grid-cols-[minmax(0,2fr)_120px_minmax(0,2fr)_140px_40px] gap-4 border-b border-line bg-[var(--surface-2)]/40 px-5 py-2 text-[9px] font-semibold uppercase tracking-[0.12em] text-subtle lg:grid">
              <span>Account</span>
              <span>Role</span>
              <span>Share access</span>
              <span>Last active</span>
              <span />
            </div>
            <div className="divide-y divide-line/60">
              {filtered.map((user) => (
                <div
                  key={user.name}
                  className="grid grid-cols-1 gap-3 px-5 py-3.5 transition hover:bg-[var(--surface-2)]/40 lg:grid-cols-[minmax(0,2fr)_120px_minmax(0,2fr)_140px_40px] lg:items-center lg:gap-4"
                >
                  <button type="button" onClick={() => setSelectedName(user.name)} className="flex min-w-0 items-center gap-3 text-left">
                    <EntityIcon src={userIconURL(user.name)} name={user.name} size={34} />
                    <span className="min-w-0">
                      <span className="flex items-center gap-1.5">
                        <span className="truncate text-[12.5px] font-medium text-ink">{user.name}</span>
                        {user.root ? <Badge tone="accent">root</Badge> : null}
                      </span>
                      <span className="mt-0.5 block truncate text-[10px] text-subtle">{user.description}</span>
                    </span>
                  </button>

                  <div>
                    <Badge tone={user.role === "administrator" ? "violet" : user.role === "disabled" ? "neutral" : "info"}>
                      {user.role}
                    </Badge>
                  </div>

                  <div className="flex flex-wrap gap-1.5">
                    {(user.shares ?? []).length === 0 ? (
                      <span className="text-[10px] text-subtle">No access</span>
                    ) : (user.shares ?? [])[0] === "all" ? (
                      <Badge tone="neutral">All shares</Badge>
                    ) : (
                      <>
                        {(user.shares ?? []).slice(0, 2).map((share) => (
                          <span key={share} className="chip text-[9.5px]">{share}</span>
                        ))}
                        {(user.shares ?? []).length > 2 ? <span className="text-[9.5px] text-subtle">+{(user.shares ?? []).length - 2}</span> : null}
                      </>
                    )}
                  </div>

                  <div className="flex items-center gap-1.5 text-[10.5px] text-muted">
                    <Clock3 className="size-3 text-subtle" />
                    {user.lastActive ? relativeTime(user.lastActive) : "never"}
                  </div>

                  <div className="flex justify-end">
                    <Menu
                      trigger={
                        <IconButton label={`Actions for ${user.name}`} size="sm">
                          <span className="text-[15px] leading-none">⋯</span>
                        </IconButton>
                      }
                      items={[
                        { label: "View profile", onSelect: () => setSelectedName(user.name) },
                        { label: "Share permissions", onSelect: () => setSelectedName(user.name) },
                      ]}
                    />
                  </div>
                </div>
              ))}
            </div>
          </>
        )}
      </Card>

      <div className="mt-4 grid grid-cols-1 gap-4 xl:grid-cols-[minmax(0,1fr)_340px]">
        <Card className="card-pad">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <h2 className="text-[13px] font-semibold text-ink">Share access matrix</h2>
              <p className="mt-0.5 text-[10.5px] text-subtle">Which account can reach which share</p>
            </div>
            <Badge tone="neutral">{snapshot.shares.length} shares</Badge>
          </div>
          <div className="mt-4 overflow-x-auto">
            <table className="w-full min-w-[560px] text-left">
              <thead>
                <tr className="border-b border-line">
                  <th className="pb-2 text-[9px] font-semibold uppercase tracking-[0.12em] text-subtle">Share</th>
                  {users.map((user) => (
                    <th key={user.name} className="pb-2 text-center text-[9px] font-semibold uppercase tracking-[0.1em] text-subtle">
                      {user.name}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody className="divide-y divide-line/60">
                {snapshot.shares.slice(0, 7).map((share) => (
                  <tr key={share.name} className="text-[11px]">
                    <td className="py-2.5">
                      <span className="flex items-center gap-2 text-soft"><FolderTree className="size-3.5 text-subtle" /> {share.name}</span>
                    </td>
                    {users.map((user) => {
                      const access = (user.shares ?? []).includes("all") || (user.shares ?? []).includes(share.name);
                      return (
                        <td key={user.name} className="py-2.5 text-center">
                          {access ? (
                            <span className="inline-grid size-5 place-items-center rounded-full bg-[var(--success-soft)] text-[var(--success)]">
                              <Check className="size-3" />
                            </span>
                          ) : (
                            <span className="inline-grid size-5 place-items-center rounded-full text-subtle/50">
                              <Minus className="size-3" />
                            </span>
                          )}
                        </td>
                      );
                    })}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Card>

        <Card className="card-pad">
          <h2 className="text-[13px] font-semibold text-ink">Access model</h2>
          <p className="mt-0.5 text-[10.5px] text-subtle">How permissions are applied</p>
          <div className="mt-4 space-y-3">
            <div className="rounded-xl border border-line bg-[var(--surface-2)]/50 p-3">
              <div className="flex items-center justify-between">
                <span className="text-[11px] font-medium text-soft">Administrators</span>
                <Badge tone="violet">{admins.length}</Badge>
              </div>
              <p className="mt-1 text-[10px] leading-relaxed text-muted">Full web interface access, array control and settings.</p>
            </div>
            <div className="rounded-xl border border-line bg-[var(--surface-2)]/50 p-3">
              <div className="flex items-center justify-between">
                <span className="text-[11px] font-medium text-soft">Read-only</span>
                <Badge tone="info">{readOnly.length}</Badge>
              </div>
              <p className="mt-1 text-[10px] leading-relaxed text-muted">Scoped share access without array or system changes.</p>
            </div>
            <div className="rounded-xl border border-line bg-[var(--surface-2)]/50 p-3">
              <div className="flex items-center justify-between">
                <span className="text-[11px] font-medium text-soft">Disabled</span>
                <Badge tone="neutral">{users.filter((user) => user.role === "disabled").length}</Badge>
              </div>
              <p className="mt-1 text-[10px] leading-relaxed text-muted">Retained accounts that cannot authenticate.</p>
            </div>
          </div>
          <div className="mt-3 flex items-start gap-2 rounded-xl border border-line p-3 text-[10px] leading-relaxed text-muted">
            <ShieldCheck className="mt-0.5 size-3.5 shrink-0 text-[var(--success)]" />
            Passwords are validated by Unraid; Aurora never stores credentials.
          </div>
        </Card>
      </div>

      <Drawer
        open={Boolean(selected)}
        onClose={() => setSelectedName(null)}
        title={selected?.name ?? ""}
        subtitle={selected?.description}
        footer={
          selected ? (
            <div className="flex flex-wrap gap-2">
              <Button
                variant="secondary"
                icon={<KeyRound className="size-3.5" />}
                onClick={() => {
                  setPassword("");
                  setPasswordConfirm("");
                  setError("");
                  setPasswordOpen(true);
                }}
              >
                Change password
              </Button>
              <Button
                variant="secondary"
                icon={<UserCog className="size-3.5" />}
                onClick={() => {
                  setDescription(selected.description);
                  setError("");
                  setEditOpen(true);
                }}
              >
                Edit
              </Button>
              {selected.root ? null : (
                <Button
                  variant="danger"
                  icon={<Trash2 className="size-3.5" />}
                  onClick={() => {
                    setError("");
                    setDeleteOpen(true);
                  }}
                >
                  Delete
                </Button>
              )}
            </div>
          ) : null
        }
      >
        {selected ? (
          <div className="space-y-5">
            <div className="flex items-center gap-4 rounded-xl border border-line bg-[var(--surface-2)]/60 p-4">
              <EntityIcon src={userIconURL(selected.name)} name={selected.name} size={52} />
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <p className="text-[14px] font-semibold text-ink">{selected.name}</p>
                  <Badge tone={selected.role === "administrator" ? "violet" : "info"}>{selected.role}</Badge>
                </div>
                <p className="mt-1 text-[10.5px] text-muted">UID {selected.uid} · member of users</p>
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <Info label="Role" value={selected.role} />
              <Info label="UID" value={String(selected.uid)} />
              <Info label="Last active" value={selected.lastActive ? formatDateTime(selected.lastActive) : "Never"} />
              <Info label="Account type" value={selected.root ? "System" : "Local"} />
            </div>

            <div>
              <h3 className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.12em] text-subtle">
                <KeyRound className="size-3.5" /> Share access
              </h3>
              <div className="mt-2 overflow-hidden rounded-xl border border-line">
                {(selected.shares ?? []).length === 0 ? (
                  <p className="px-3 py-3 text-[11px] text-muted">This account has no share access.</p>
                ) : (selected.shares ?? [])[0] === "all" ? (
                  <div className="flex items-center justify-between px-3 py-2.5 text-[11px]">
                    <span className="text-soft">All shares</span>
                    <Badge tone="success">read / write</Badge>
                  </div>
                ) : (
                  (selected.shares ?? []).map((share) => (
                    <div key={share} className="flex items-center justify-between border-b border-line/60 px-3 py-2.5 text-[11px] last:border-0">
                      <span className="flex items-center gap-2 text-soft"><FolderTree className="size-3.5 text-subtle" /> {share}</span>
                      <Badge tone="info">{selected.role === "read-only" ? "read only" : "read / write"}</Badge>
                    </div>
                  ))
                )}
              </div>
            </div>

            <div className="rounded-xl border border-line bg-[var(--surface-2)]/50 p-3 text-[10.5px] leading-relaxed text-muted">
              <p className="flex items-center gap-1.5 font-medium text-soft"><UserCog className="size-3.5" /> Account management</p>
              <p className="mt-1.5">
                Accounts are created through Unraid's own user manager. Passwords are sent once to emhttpd,
                never stored by Aurora.
              </p>
            </div>
          </div>
        ) : null}
      </Drawer>

      <Modal
        open={addOpen}
        onClose={() => setAddOpen(false)}
        title="Add user"
        description="Creates a Unix account, a Samba account and a share access entry, like the official Add User page."
        footer={
          <>
            <Button variant="secondary" onClick={() => setAddOpen(false)} disabled={createUser.isPending}>
              Cancel
            </Button>
            <Button
              variant="primary"
              loading={createUser.isPending}
              onClick={() => {
                setError("");
                if (!/^[a-z_][a-z0-9_-]{0,39}$/.test(addDraft.name)) {
                  setError("Use a lowercase name starting with a letter or underscore.");
                  return;
                }
                if (!addDraft.password) {
                  setError("A password is required.");
                  return;
                }
                if (addDraft.password !== addDraft.confirm) {
                  setError("Passwords do not match.");
                  return;
                }
                createUser.mutate({
                  name: addDraft.name,
                  description: addDraft.description,
                  password: addDraft.password,
                });
              }}
            >
              Create user
            </Button>
          </>
        }
      >
        <div className="space-y-3">
          <Field label="User name" value={addDraft.name} onChange={(value) => setAddDraft((current) => ({ ...current, name: value }))} placeholder="jellyfin" />
          <Field label="Description" value={addDraft.description} onChange={(value) => setAddDraft((current) => ({ ...current, description: value }))} placeholder="Media account" />
          <Field label="Password" type="password" value={addDraft.password} onChange={(value) => setAddDraft((current) => ({ ...current, password: value }))} />
          <Field label="Confirm password" type="password" value={addDraft.confirm} onChange={(value) => setAddDraft((current) => ({ ...current, confirm: value }))} />
          {error ? <ErrorLine message={error} /> : null}
        </div>
      </Modal>

      <Modal
        open={editOpen}
        onClose={() => setEditOpen(false)}
        title={`Edit · ${selected?.name ?? ""}`}
        footer={
          <>
            <Button variant="secondary" onClick={() => setEditOpen(false)} disabled={updateUser.isPending}>
              Cancel
            </Button>
            <Button
              variant="primary"
              loading={updateUser.isPending}
              onClick={() => {
                setError("");
                if (selectedName) updateUser.mutate({ name: selectedName, value: description });
              }}
            >
              Save
            </Button>
          </>
        }
      >
        <div className="space-y-3">
          <Field label="Description" value={description} onChange={setDescription} />
          {error ? <ErrorLine message={error} /> : null}
        </div>
      </Modal>

      <Modal
        open={passwordOpen}
        onClose={() => setPasswordOpen(false)}
        title={`Change password · ${selected?.name ?? ""}`}
        footer={
          <>
            <Button variant="secondary" onClick={() => setPasswordOpen(false)} disabled={changePassword.isPending}>
              Cancel
            </Button>
            <Button
              variant="primary"
              loading={changePassword.isPending}
              onClick={() => {
                setError("");
                if (!password) {
                  setError("Enter a new password.");
                  return;
                }
                if (password !== passwordConfirm) {
                  setError("Passwords do not match.");
                  return;
                }
                if (selectedName) changePassword.mutate({ name: selectedName, value: password });
              }}
            >
              Change password
            </Button>
          </>
        }
      >
        <div className="space-y-3">
          <Field label="New password" type="password" value={password} onChange={setPassword} />
          <Field label="Confirm password" type="password" value={passwordConfirm} onChange={setPasswordConfirm} />
          {error ? <ErrorLine message={error} /> : null}
        </div>
      </Modal>

      <Modal
        open={deleteOpen}
        onClose={() => setDeleteOpen(false)}
        title={`Delete ${selected?.name ?? "user"}?`}
        description="The account, its Samba credentials and share access entries are removed."
        footer={
          <>
            <Button variant="secondary" onClick={() => setDeleteOpen(false)} disabled={deleteUser.isPending}>
              Cancel
            </Button>
            <Button
              variant="danger"
              icon={<Trash2 className="size-3.5" />}
              loading={deleteUser.isPending}
              onClick={() => {
                if (selectedName) deleteUser.mutate(selectedName);
              }}
            >
              Delete user
            </Button>
          </>
        }
      >
        <div className="space-y-3">
          <ErrorLine message={error} />
        </div>
      </Modal>
    </>
  );
}

function Field({
  label,
  value,
  onChange,
  placeholder,
  type = "text",
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  type?: string;
}) {
  return (
    <label className="block space-y-1">
      <span className="block text-[9.5px] uppercase tracking-[0.08em] text-subtle">{label}</span>
      <input
        type={type}
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
        className="h-9 w-full rounded-lg border border-line bg-[var(--surface-2)] px-2.5 text-[11px] text-ink outline-none focus:border-[var(--accent-line)]"
      />
    </label>
  );
}

function ErrorLine({ message }: { message: string }) {
  if (!message) return null;
  return (
    <p className="rounded-lg border border-[color-mix(in_srgb,var(--danger)_24%,transparent)] bg-[var(--danger-soft)] px-3 py-2 text-[10.5px] text-[var(--danger)]">
      {message}
    </p>
  );
}

function Stat({ icon, label, value, hint }: { icon: ReactNode; label: string; value: string; hint: string }) {
  return (
    <Card className="card-pad">
      <div className="flex items-center gap-2">
        <span className="grid size-6 place-items-center rounded-md border border-line bg-[var(--surface-2)] text-muted">{icon}</span>
        <p className="text-[10px] font-medium uppercase tracking-[0.1em] text-subtle">{label}</p>
      </div>
      <p className="num mt-3 text-[21px] font-semibold leading-none text-ink">{value}</p>
      <p className="mt-1.5 text-[10px] text-muted">{hint}</p>
    </Card>
  );
}

function Info({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-line bg-[var(--surface-2)]/50 px-3 py-2">
      <p className="text-[9px] uppercase tracking-[0.1em] text-subtle">{label}</p>
      <p className="mt-1 truncate text-[11px] text-soft">{value}</p>
    </div>
  );
}

