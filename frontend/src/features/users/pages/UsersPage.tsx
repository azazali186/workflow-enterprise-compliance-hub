import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { MoreVertical, Plus, Search, UserPlus } from "lucide-react";
import { useMemo, useState } from "react";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { DataTable, type Column } from "@/components/ui/DataTable";
import { Dropdown } from "@/components/ui/Dropdown";
import { Field } from "@/components/ui/Field";
import { Input } from "@/components/ui/Input";
import { PaginationBar } from "@/components/ui/PaginationBar";
import { PageHeader } from "@/components/ui/PageHeader";
import { Select } from "@/components/ui/Select";
import { DateRangeInputs, type DateRangeValue } from "@/components/ui/Toolbar";
import { Avatar } from "@/components/ui/Avatar";
import { useAuth } from "@/hooks/useAuth";
import { useCursorList } from "@/hooks/useCursorList";
import { useDebouncedValue } from "@/hooks/useDebounce";
import { statusMeta } from "@/lib/constants";
import { formatDate, shortId, smartDateTime } from "@/lib/format";
import { PERM } from "@/services/api/paths";
import { rolesApi } from "@/services/roles.service";
import { usersApi } from "@/services/users.service";
import { toast } from "@/store/toast.slice";
import { ApiError, type Paginated } from "@/types/api";
import { USER_STATUSES, type User } from "@/types/entities";
import { UserFormModal } from "../components/UserFormModal";

function dateToRange(value: DateRangeValue): { field: string; from?: string; to?: string } | undefined {
  if (!value.from && !value.to) return undefined;
  return {
    field: "created_at",
    from: value.from ? `${value.from}T00:00:00.000Z` : undefined,
    to: value.to ? `${value.to}T23:59:59.999Z` : undefined,
  };
}

export function UsersPage() {
  const { can } = useAuth();
  const qc = useQueryClient();

  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(search, 350);
  const [statusFilter, setStatusFilter] = useState("");
  const [roleFilter, setRoleFilter] = useState("");
  const [dateRange, setDateRange] = useState<DateRangeValue>({});
  const [sort, setSort] = useState<{ column: string; direction: "asc" | "desc" } | undefined>(undefined);
  const [createOpen, setCreateOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<User | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<User | null>(null);

  const rolesQuery = useQuery({
    queryKey: ["roles"],
    queryFn: ({ signal }) => rolesApi.listAll(signal),
    staleTime: 5 * 60_000,
  });
  const roleById = useMemo(() => new Map((rolesQuery.data ?? []).map((r) => [r.id, r])), [rolesQuery.data]);

  const filters = useMemo(() => {
    const f: Record<string, unknown> = {};
    if (debouncedSearch) f.username = debouncedSearch;
    if (statusFilter) f.status = statusFilter;
    if (roleFilter) f.role_id = roleFilter;
    return f;
  }, [debouncedSearch, statusFilter, roleFilter]);

  const list = useCursorList<User>({
    queryKey: ["users-list"],
    queryFn: usersApi.search,
    limit: 10,
    sort,
    filters,
    dateRange: dateToRange(dateRange),
  });

  const canCreate = can(PERM.usersCreate);
  const canUpdate = can(PERM.usersUpdate);
  const canDelete = can(PERM.usersDelete);

  /* --- mutations --- */
  const createUser = useMutation({
    mutationFn: usersApi.create,
    onSuccess: (u) => {
      toast.success("User created", u.username);
      void qc.invalidateQueries({ queryKey: ["users-list"] });
    },
    onError: (e) => toast.error("Could not create user", ApiError.is(e) ? e.userMessage : undefined),
  });

  const updateUser = useMutation({
    mutationFn: usersApi.update,
    onSuccess: (u) => {
      toast.success("User updated", u.username);
      void qc.invalidateQueries({ queryKey: ["users-list"] });
    },
    onError: (e) => toast.error("Could not update user", ApiError.is(e) ? e.userMessage : undefined),
  });

  const deleteUser = useMutation({
    mutationFn: (id: string) => usersApi.remove(id),
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: ["users-list"] });
      const snapshot = qc.getQueriesData<Paginated<User>>({ queryKey: ["users-list"] });
      qc.setQueriesData<Paginated<User>>({ queryKey: ["users-list"] }, (old) =>
        old ? { ...old, items: old.items.filter((u) => u.id !== id), pagination: { ...old.pagination, count: old.pagination.count - 1 } } : old,
      );
      return snapshot;
    },
    onError: (e, _id, snapshot) => {
      if (snapshot) for (const [key, data] of snapshot) if (data) qc.setQueryData(key, data);
      toast.error("Could not delete user", ApiError.is(e) ? e.userMessage : undefined);
    },
    onSuccess: () => toast.success("User deleted"),
    onSettled: () => void qc.invalidateQueries({ queryKey: ["users-list"] }),
  });

  const columns: Column<User>[] = [
    {
      key: "username",
      header: "User",
      sortable: true,
      className: "min-w-44",
      render: (u) => (
        <div className="flex items-center gap-3">
          <Avatar name={u.username} />
          <div className="min-w-0">
            <p className="truncate font-medium text-slate-900">{u.username}</p>
            <p className="truncate text-xs text-slate-500">{u.email || "—"}</p>
          </div>
        </div>
      ),
    },
    {
      key: "role_id",
      header: "Role",
      sortable: true,
      render: (u) => {
        const role = roleById.get(u.role_id ?? "");
        return <span className="text-slate-700">{role?.name ?? shortId(u.role_id)}</span>;
      },
    },
    {
      key: "status",
      header: "Status",
      sortable: true,
      render: (u) => {
        const meta = statusMeta[u.status] ?? { label: u.status, tone: "neutral" as const };
        return <Badge tone={meta.tone}>{meta.label}</Badge>;
      },
    },
    {
      key: "last_login_at",
      header: "Last login",
      hideBelow: "md",
      render: (u) => <span className="text-slate-600">{formatLogin(u.last_login_at)}</span>,
    },
    { key: "created_at", header: "Created", sortable: true, hideBelow: "lg", render: (u) => <span className="text-slate-600">{formatCreated(u.created_at)}</span> },
  ];

  const hasFilters = Boolean(debouncedSearch || statusFilter || roleFilter || dateRange.from || dateRange.to);

  return (
    <div className="space-y-5">
      <PageHeader
        title="Users"
        description="Manage operator accounts and their roles."
        actions={
          canCreate ? (
            <Button icon={<Plus className="h-4 w-4" />} onClick={() => setCreateOpen(true)}>
              New user
            </Button>
          ) : undefined
        }
      />

      <DataTable
        columns={columns}
        rows={list.page?.items}
        keyExtractor={(u) => u.id}
        isLoading={list.isLoading}
        isError={list.isError}
        error={list.error}
        onRetry={list.refetch}
        sort={sort}
        onSortChange={setSort}
        emptyTitle="No users found"
        emptyDescription={
          hasFilters ? "Try adjusting or clearing the filters below." : "Invite your first operator to get started."
        }
        toolbar={
          <div className="flex flex-wrap items-end gap-2.5">
            <Field label="Filter by username" htmlFor="users-search">
              <div className="relative">
                <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
                <Input
                  id="users-search"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  placeholder="Exact username"
                  className="w-52 pl-9"
                />
              </div>
            </Field>
            <Field label="Status" htmlFor="users-status">
              <Select
                id="users-status"
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value)}
                placeholder="All statuses"
                options={USER_STATUSES.map((s) => ({ value: s, label: statusMeta[s]?.label ?? s }))}
                className="w-36"
              />
            </Field>
            <Field label="Role" htmlFor="users-role">
              <Select
                id="users-role"
                value={roleFilter}
                onChange={(e) => setRoleFilter(e.target.value)}
                placeholder="All roles"
                options={(rolesQuery.data ?? []).map((r) => ({ value: r.id, label: r.name }))}
                className="w-44"
              />
            </Field>
            <DateRangeInputs value={dateRange} onChange={setDateRange} />
            {hasFilters && (
              <button
                type="button"
                onClick={() => {
                  setSearch("");
                  setStatusFilter("");
                  setRoleFilter("");
                  setDateRange({});
                }}
                className="inline-flex h-9.5 items-center rounded-lg px-3 text-sm font-medium text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-700"
              >
                Clear filters
              </button>
            )}
          </div>
        }
        rowActions={(u) =>
          canUpdate || canDelete ? (
            <Dropdown
              trigger={({ toggle }) => (
                <button
                  type="button"
                  onClick={toggle}
                  className="rounded-md p-1.5 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700"
                  aria-label={`Actions for ${u.username}`}
                >
                  <MoreVertical className="h-4 w-4" />
                </button>
              )}
              items={[
                ...(canUpdate
                  ? [{ key: "edit", label: "Edit user", icon: <UserPlus className="h-4 w-4" aria-hidden="true" />, onSelect: () => setEditTarget(u) }]
                  : []),
                ...(canDelete
                  ? [{ key: "delete", label: "Delete user", danger: true, icon: undefined, onSelect: () => setDeleteTarget(u) }]
                  : []),
              ]}
            />
          ) : undefined
        }
      />

      <PaginationBar
        rangeText={list.rangeText}
        pageNumber={list.pageNumber}
        isFirstPage={list.isFirstPage}
        hasMore={list.hasMore}
        isFetching={list.isFetching}
        isLoading={list.isLoading}
        onPrevious={list.loadPrevious}
        onNext={list.loadNext}
      />

      <UserFormModal
        open={createOpen || Boolean(editTarget)}
        onClose={() => {
          setCreateOpen(false);
          setEditTarget(null);
        }}
        user={editTarget}
        roles={rolesQuery.data ?? []}
        rolesLoading={rolesQuery.isPending}
        canUpdate={canUpdate}
        onSubmit={async (values) => {
          if (editTarget) await updateUser.mutateAsync({ id: editTarget.id, ...values });
          else await createUser.mutateAsync(values);
          setCreateOpen(false);
          setEditTarget(null);
        }}
        submitting={createUser.isPending || updateUser.isPending}
      />

      <ConfirmDialog
        open={Boolean(deleteTarget)}
        title="Delete user"
        description={
          deleteTarget
            ? `"${deleteTarget.username}" will be deactivated immediately and their active session invalidated. This can be undone by an administrator if needed.`
            : ""
        }
        loading={deleteUser.isPending}
        onClose={() => setDeleteTarget(null)}
        onConfirm={() => {
          if (deleteTarget) void deleteUser.mutate(deleteTarget.id);
          setDeleteTarget(null);
        }}
      />
    </div>
  );
}

function formatLogin(iso?: string | null): string {
  return iso ? smartDateTime(iso) : "Never";
}

function formatCreated(iso?: string | null): string {
  return formatDate(iso);
}
