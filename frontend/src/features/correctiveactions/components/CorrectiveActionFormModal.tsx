import { useEffect, useState, type FormEvent } from "react";
import { Button } from "@/components/ui/Button";
import { Field } from "@/components/ui/Field";
import { Input } from "@/components/ui/Input";
import { Modal } from "@/components/ui/Modal";
import { SearchableSelect } from "@/components/ui/SearchableSelect";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { statusMeta } from "@/lib/constants";
import { ApiError } from "@/types/api";
import { CORRECTIVE_ACTION_STATUSES, type CorrectiveAction } from "@/types/entities";

export interface CorrectiveActionFormValues {
  title: string;
  description: string;
  status: string;
  violation_id: string;
  owner_id: string;
  due_date: string | null;
}

export interface CorrectiveActionFormModalProps {
  open: boolean;
  onClose: () => void;
  action: CorrectiveAction | null;
  onSubmit: (values: CorrectiveActionFormValues) => Promise<void>;
  submitting: boolean;
}

export function CorrectiveActionFormModal({ open, onClose, action, onSubmit, submitting }: CorrectiveActionFormModalProps) {
  const isEdit = Boolean(action);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [status, setStatus] = useState("open");
  const [violationId, setViolationId] = useState("");
  const [ownerId, setOwnerId] = useState("");
  const [dueDate, setDueDate] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [submitError, setSubmitError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setTitle(action?.title ?? "");
    setDescription(action?.description ?? "");
    setStatus(action?.status ?? "open");
    setViolationId(action?.violation_id ?? "");
    setOwnerId(action?.owner_id ?? "");
    setDueDate(action?.due_date ? action.due_date.slice(0, 10) : "");
    setErrors({});
    setSubmitError(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, action?.id]);

  const submit = async (ev: FormEvent) => {
    ev.preventDefault();
    const e: Record<string, string> = {};
    if (!title.trim()) e.title = "Title is required.";
    setErrors(e);
    if (Object.keys(e).length > 0) return;
    setSubmitError(null);
    try {
      await onSubmit({
        title: title.trim(),
        description,
        status,
        violation_id: violationId.trim(),
        owner_id: ownerId.trim(),
        due_date: dueDate ? `${dueDate}T23:59:59.999Z` : null,
      });
    } catch (err) {
      setSubmitError(ApiError.is(err) ? err.userMessage : "Something went wrong. Please try again.");
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={isEdit ? "Edit corrective action" : "New corrective action"}
      description="A remediation plan for a violation, with owner and due date."
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={submitting}>
            Cancel
          </Button>
          <Button type="submit" form="ca-form" loading={submitting}>
            {isEdit ? "Save changes" : "Create action"}
          </Button>
        </>
      }
    >
      <form id="ca-form" onSubmit={submit} className="space-y-4" noValidate>
        {submitError && (
          <div className="rounded-lg border border-danger-200 bg-danger-50 px-3.5 py-2.5 text-sm text-danger-700" role="alert">
            {submitError}
          </div>
        )}

        <Field label="Title" htmlFor="ca-title" required error={errors.title}>
          <Input id="ca-title" value={title} onChange={(e) => setTitle(e.target.value)} autoFocus />
        </Field>

        <Field label="Description" htmlFor="ca-description">
          <Textarea id="ca-description" rows={3} value={description} onChange={(e) => setDescription(e.target.value)} />
        </Field>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Status" htmlFor="ca-status">
            <Select
              id="ca-status"
              value={status}
              onChange={(e) => setStatus(e.target.value)}
              options={CORRECTIVE_ACTION_STATUSES.map((s) => ({ value: s, label: statusMeta[s]?.label ?? s }))}
            />
          </Field>
          <Field label="Due date" htmlFor="ca-due">
            <Input id="ca-due" type="date" value={dueDate} onChange={(e) => setDueDate(e.target.value)} />
          </Field>
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Violation" htmlFor="ca-violation" hint="Optional violation being remediated.">
            <SearchableSelect id="ca-violation" entity="violations" value={violationId} onChange={setViolationId} placeholder="Search violation…" />
          </Field>
          <Field label="Owner" htmlFor="ca-owner" hint="Optional operator account.">
            <SearchableSelect id="ca-owner" entity="users" value={ownerId} onChange={setOwnerId} placeholder="Search user…" />
          </Field>
        </div>
      </form>
    </Modal>
  );
}
