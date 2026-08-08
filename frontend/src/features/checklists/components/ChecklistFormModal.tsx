import { useEffect, useState, type FormEvent } from "react";
import { Button } from "@/components/ui/Button";
import { Field } from "@/components/ui/Field";
import { Input } from "@/components/ui/Input";
import { Modal } from "@/components/ui/Modal";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { statusMeta } from "@/lib/constants";
import { ApiError } from "@/types/api";
import { CHECKLIST_STATUSES, type Checklist } from "@/types/entities";

export interface ChecklistFormValues {
  title: string;
  description: string;
  status: string;
  compliance_id: string;
  owner_id: string;
  due_date: string | null;
  items: unknown;
}

export interface ChecklistFormModalProps {
  open: boolean;
  onClose: () => void;
  checklist: Checklist | null;
  onSubmit: (values: ChecklistFormValues) => Promise<void>;
  submitting: boolean;
}

export function ChecklistFormModal({ open, onClose, checklist, onSubmit, submitting }: ChecklistFormModalProps) {
  const isEdit = Boolean(checklist);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [status, setStatus] = useState("open");
  const [complianceId, setComplianceId] = useState("");
  const [ownerId, setOwnerId] = useState("");
  const [dueDate, setDueDate] = useState("");
  const [itemsJson, setItemsJson] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [submitError, setSubmitError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setTitle(checklist?.title ?? "");
    setDescription(checklist?.description ?? "");
    setStatus(checklist?.status ?? "open");
    setComplianceId(checklist?.compliance_id ?? "");
    setOwnerId(checklist?.owner_id ?? "");
    setDueDate(checklist?.due_date ? checklist.due_date.slice(0, 10) : "");
    setItemsJson(checklist?.items ? JSON.stringify(checklist.items, null, 2) : "");
    setErrors({});
    setSubmitError(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, checklist?.id]);

  const submit = async (ev: FormEvent) => {
    ev.preventDefault();
    const e: Record<string, string> = {};
    if (!title.trim()) e.title = "Title is required.";
    let parsedItems: unknown;
    if (itemsJson.trim()) {
      try {
        parsedItems = JSON.parse(itemsJson);
      } catch {
        e.items = "Items must be valid JSON, e.g. [{\"label\": \"Step one\", \"done\": false}].";
      }
    }
    setErrors(e);
    if (Object.keys(e).length > 0) return;
    setSubmitError(null);
    try {
      await onSubmit({
        title: title.trim(),
        description,
        status,
        compliance_id: complianceId.trim(),
        owner_id: ownerId.trim(),
        due_date: dueDate ? `${dueDate}T23:59:59.999Z` : null,
        items: parsedItems,
      });
    } catch (err) {
      setSubmitError(ApiError.is(err) ? err.userMessage : "Something went wrong. Please try again.");
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={isEdit ? "Edit checklist" : "New checklist"}
      description="A checklist of verification steps linked to a compliance record."
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={submitting}>
            Cancel
          </Button>
          <Button type="submit" form="checklist-form" loading={submitting}>
            {isEdit ? "Save changes" : "Create checklist"}
          </Button>
        </>
      }
    >
      <form id="checklist-form" onSubmit={submit} className="space-y-4" noValidate>
        {submitError && (
          <div className="rounded-lg border border-danger-200 bg-danger-50 px-3.5 py-2.5 text-sm text-danger-700" role="alert">
            {submitError}
          </div>
        )}

        <Field label="Title" htmlFor="checklist-title" required error={errors.title}>
          <Input id="checklist-title" value={title} onChange={(e) => setTitle(e.target.value)} autoFocus />
        </Field>

        <Field label="Description" htmlFor="checklist-description">
          <Textarea id="checklist-description" rows={2} value={description} onChange={(e) => setDescription(e.target.value)} />
        </Field>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Status" htmlFor="checklist-status">
            <Select
              id="checklist-status"
              value={status}
              onChange={(e) => setStatus(e.target.value)}
              options={CHECKLIST_STATUSES.map((s) => ({ value: s, label: statusMeta[s]?.label ?? s }))}
            />
          </Field>
          <Field label="Due date" htmlFor="checklist-due">
            <Input id="checklist-due" type="date" value={dueDate} onChange={(e) => setDueDate(e.target.value)} />
          </Field>
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Compliance id" htmlFor="checklist-compliance" hint="Optional UUID of the compliance record.">
            <Input id="checklist-compliance" value={complianceId} onChange={(e) => setComplianceId(e.target.value)} placeholder="00000000-0000-0000-0000-000000000000" />
          </Field>
          <Field label="Owner" htmlFor="checklist-owner" hint="Optional owner id.">
            <Input id="checklist-owner" value={ownerId} onChange={(e) => setOwnerId(e.target.value)} />
          </Field>
        </div>

        <Field label="Items (JSON)" htmlFor="checklist-items" error={errors.items} hint="Optional array of {label, done} steps.">
          <Textarea id="checklist-items" rows={4} value={itemsJson} onChange={(e) => setItemsJson(e.target.value)} placeholder='[{"label": "Verify access controls", "done": false}]' className="font-mono text-xs" />
        </Field>
      </form>
    </Modal>
  );
}
