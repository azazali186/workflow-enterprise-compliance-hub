import { useEffect, useState, type FormEvent } from "react";
import { Button } from "@/components/ui/Button";
import { Field } from "@/components/ui/Field";
import { Input } from "@/components/ui/Input";
import { Modal } from "@/components/ui/Modal";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { statusMeta } from "@/lib/constants";
import { ApiError } from "@/types/api";
import { DEADLINE_STATUSES, type Deadline } from "@/types/entities";

export interface DeadlineFormValues {
  title: string;
  description: string;
  status: string;
  entity_type: string;
  entity_id: string;
  deadline_at: string;
}

export interface DeadlineFormModalProps {
  open: boolean;
  onClose: () => void;
  deadline: Deadline | null;
  onSubmit: (values: DeadlineFormValues) => Promise<void>;
  submitting: boolean;
}

export function DeadlineFormModal({ open, onClose, deadline, onSubmit, submitting }: DeadlineFormModalProps) {
  const isEdit = Boolean(deadline);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [status, setStatus] = useState("upcoming");
  const [entityType, setEntityType] = useState("");
  const [entityId, setEntityId] = useState("");
  const [deadlineAt, setDeadlineAt] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [submitError, setSubmitError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setTitle(deadline?.title ?? "");
    setDescription(deadline?.description ?? "");
    setStatus(deadline?.status ?? "upcoming");
    setEntityType(deadline?.entity_type ?? "");
    setEntityId(deadline?.entity_id ?? "");
    setDeadlineAt(deadline?.deadline_at ? deadline.deadline_at.slice(0, 16) : "");
    setErrors({});
    setSubmitError(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, deadline?.id]);

  const submit = async (ev: FormEvent) => {
    ev.preventDefault();
    const e: Record<string, string> = {};
    if (!title.trim()) e.title = "Title is required.";
    if (!deadlineAt) e.deadline_at = "Deadline time is required.";
    setErrors(e);
    if (Object.keys(e).length > 0) return;
    setSubmitError(null);
    try {
      await onSubmit({
        title: title.trim(),
        description,
        status,
        entity_type: entityType.trim(),
        entity_id: entityId.trim(),
        deadline_at: `${deadlineAt}:00.000Z`,
      });
    } catch (err) {
      setSubmitError(ApiError.is(err) ? err.userMessage : "Something went wrong. Please try again.");
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={isEdit ? "Edit deadline" : "New deadline"}
      description="A compliance deadline evaluated by the deadline job (due/overdue transitions)."
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={submitting}>
            Cancel
          </Button>
          <Button type="submit" form="deadline-form" loading={submitting}>
            {isEdit ? "Save changes" : "Create deadline"}
          </Button>
        </>
      }
    >
      <form id="deadline-form" onSubmit={submit} className="space-y-4" noValidate>
        {submitError && (
          <div className="rounded-lg border border-danger-200 bg-danger-50 px-3.5 py-2.5 text-sm text-danger-700" role="alert">
            {submitError}
          </div>
        )}

        <Field label="Title" htmlFor="deadline-title" required error={errors.title}>
          <Input id="deadline-title" value={title} onChange={(e) => setTitle(e.target.value)} autoFocus />
        </Field>

        <Field label="Description" htmlFor="deadline-description">
          <Textarea id="deadline-description" rows={2} value={description} onChange={(e) => setDescription(e.target.value)} />
        </Field>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Deadline" htmlFor="deadline-at" required error={errors.deadline_at}>
            <Input id="deadline-at" type="datetime-local" value={deadlineAt} onChange={(e) => setDeadlineAt(e.target.value)} />
          </Field>
          <Field label="Status" htmlFor="deadline-status">
            <Select
              id="deadline-status"
              value={status}
              onChange={(e) => setStatus(e.target.value)}
              options={DEADLINE_STATUSES.map((s) => ({ value: s, label: statusMeta[s]?.label ?? s }))}
            />
          </Field>
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Entity type" htmlFor="deadline-entity-type" hint="e.g. compliance, audit.">
            <Input id="deadline-entity-type" value={entityType} onChange={(e) => setEntityType(e.target.value)} />
          </Field>
          <Field label="Entity id" htmlFor="deadline-entity-id" hint="Optional UUID the deadline belongs to.">
            <Input id="deadline-entity-id" value={entityId} onChange={(e) => setEntityId(e.target.value)} placeholder="00000000-0000-0000-0000-000000000000" />
          </Field>
        </div>
      </form>
    </Modal>
  );
}
