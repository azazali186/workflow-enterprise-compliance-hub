import { useEffect, useState, type FormEvent } from "react";
import { Button } from "@/components/ui/Button";
import { Field } from "@/components/ui/Field";
import { Input } from "@/components/ui/Input";
import { Modal } from "@/components/ui/Modal";
import { SearchableSelect } from "@/components/ui/SearchableSelect";
import { Select } from "@/components/ui/Select";
import { statusMeta } from "@/lib/constants";
import { ApiError } from "@/types/api";
import { USER_STATUSES, type User } from "@/types/entities";

export interface UserFormValues {
  username: string;
  email: string;
  password: string;
  role_id: string;
  status: string;
}

export interface UserFormModalProps {
  open: boolean;
  onClose: () => void;
  user: User | null;
  canUpdate: boolean;
  onSubmit: (values: UserFormValues) => Promise<void>;
  submitting: boolean;
}

export function UserFormModal({ open, onClose, user, canUpdate, onSubmit, submitting }: UserFormModalProps) {
  const isEdit = Boolean(user);
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [roleId, setRoleId] = useState("");
  const [status, setStatus] = useState("active");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [submitError, setSubmitError] = useState<string | null>(null);

  // Seed the form when the modal opens for a given user.
  useEffect(() => {
    if (!open) return;
    setUsername(user?.username ?? "");
    setEmail(user?.email ?? "");
    setPassword("");
    setRoleId(user?.role_id ?? "");
    setStatus(user?.status ?? "active");
    setErrors({});
    setSubmitError(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, user?.id]);

  const validate = (): boolean => {
    const e: Record<string, string> = {};
    if (!username.trim()) e.username = "Username is required.";
    if (!roleId) e.role_id = "A role is required — every account needs a role.";
    if (isEdit) {
      if (password && password.length < 8) e.password = "Password must be at least 8 characters.";
    } else if (password.length < 8) {
      e.password = "Password must be at least 8 characters.";
    }
    if (email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) e.email = "Enter a valid email address.";
    setErrors(e);
    return Object.keys(e).length === 0;
  };

  const submit = async (ev: FormEvent) => {
    ev.preventDefault();
    if (!validate()) return;
    setSubmitError(null);
    try {
      await onSubmit({ username: username.trim(), email, password, role_id: roleId, status });
    } catch (err) {
      setSubmitError(ApiError.is(err) ? err.userMessage : "Something went wrong. Please try again.");
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={isEdit ? "Edit user" : "New user"}
      description={isEdit ? `Update ${user?.username ?? "this account"}.` : "Create an operator account with a role."}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={submitting}>
            Cancel
          </Button>
          <Button type="submit" form="user-form" loading={submitting}>
            {isEdit ? "Save changes" : "Create user"}
          </Button>
        </>
      }
    >
      <form id="user-form" onSubmit={submit} className="space-y-4" noValidate>
        {submitError && (
          <div className="rounded-lg border border-danger-200 bg-danger-50 px-3.5 py-2.5 text-sm text-danger-700" role="alert">
            {submitError}
          </div>
        )}

        <Field label="Username" htmlFor="user-username" required error={errors.username}>
          <Input id="user-username" value={username} onChange={(e) => setUsername(e.target.value)} autoFocus />
        </Field>

        <Field label="Email" htmlFor="user-email" error={errors.email} hint="Optional — used for operator contact.">
          <Input id="user-email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
        </Field>

        <Field
          label={isEdit ? "New password" : "Password"}
          htmlFor="user-password"
          required={!isEdit}
          error={errors.password}
          hint={isEdit ? "Leave blank to keep the current password." : "At least 8 characters."}
        >
          <Input id="user-password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="new-password" />
        </Field>

        <Field label="Role" htmlFor="user-role" required error={errors.role_id} hint="The role defines which routes this account can use.">
          <SearchableSelect
            id="user-role"
            entity="roles"
            value={roleId}
            onChange={setRoleId}
            placeholder="Search role…"
            invalid={Boolean(errors.role_id)}
          />
        </Field>

        {isEdit && canUpdate && (
          <Field label="Status" htmlFor="user-status" hint="Disabled accounts cannot sign in.">
            <Select
              id="user-status"
              value={status}
              onChange={(e) => setStatus(e.target.value)}
              options={USER_STATUSES.map((s) => ({ value: s, label: statusMeta[s]?.label ?? s }))}
            />
          </Field>
        )}
      </form>
    </Modal>
  );
}
