import { useEffect, useState, type FormEvent } from "react";
import { Button } from "@/components/ui/Button";
import { Field } from "@/components/ui/Field";
import { Input } from "@/components/ui/Input";
import { Modal } from "@/components/ui/Modal";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { statusMeta } from "@/lib/constants";
import { ApiError } from "@/types/api";
import { REGULATION_STATUSES, type Regulation } from "@/types/entities";

export interface RegulationFormValues {
  title: string;
  code: string;
  description: string;
  jurisdiction: string;
  status: string;
  effective_date: string | null;
  expiry_date: string | null;
}

export interface RegulationFormModalProps {
  open: boolean;
  onClose: () => void;
  regulation: Regulation | null;
  onSubmit: (values: RegulationFormValues) => Promise<void>;
  submitting: boolean;
}

export function RegulationFormModal({ open, onClose, regulation, onSubmit, submitting }: RegulationFormModalProps) {
  const isEdit = Boolean(regulation);
  const [title, setTitle] = useState("");
  const [code, setCode] = useState("");
  const [description, setDescription] = useState("");
  const [jurisdiction, setJurisdiction] = useState("");
  const [status, setStatus] = useState("active");
  const [effectiveDate, setEffectiveDate] = useState("");
  const [expiryDate, setExpiryDate] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [submitError, setSubmitError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setTitle(regulation?.title ?? "");
    setCode(regulation?.code ?? "");
    setDescription(regulation?.description ?? "");
    setJurisdiction(regulation?.jurisdiction ?? "");
    setStatus(regulation?.status ?? "active");
    setEffectiveDate(regulation?.effective_date ? regulation.effective_date.slice(0, 10) : "");
    setExpiryDate(regulation?.expiry_date ? regulation.expiry_date.slice(0, 10) : "");
    setErrors({});
    setSubmitError(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, regulation?.id]);

  const submit = async (ev: FormEvent) => {
    ev.preventDefault();
    const e: Record<string, string> = {};
    if (!title.trim()) e.title = "Title is required.";
    if (!code.trim()) e.code = "Code is required.";
    if (effectiveDate && expiryDate && expiryDate < effectiveDate) e.expiry_date = "Expiry must be after the effective date.";
    setErrors(e);
    if (Object.keys(e).length > 0) return;
    setSubmitError(null);
    try {
      await onSubmit({
        title: title.trim(),
        code: code.trim(),
        description,
        jurisdiction: jurisdiction.trim(),
        status,
        effective_date: effectiveDate ? `${effectiveDate}T00:00:00.000Z` : null,
        expiry_date: expiryDate ? `${expiryDate}T00:00:00.000Z` : null,
      });
    } catch (err) {
      setSubmitError(ApiError.is(err) ? err.userMessage : "Something went wrong. Please try again.");
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={isEdit ? "Edit regulation" : "New regulation"}
      description="A regulatory requirement — law, standard, or policy — with an optional validity window."
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={submitting}>
            Cancel
          </Button>
          <Button type="submit" form="regulation-form" loading={submitting}>
            {isEdit ? "Save changes" : "Create regulation"}
          </Button>
        </>
      }
    >
      <form id="regulation-form" onSubmit={submit} className="space-y-4" noValidate>
        {submitError && (
          <div className="rounded-lg border border-danger-200 bg-danger-50 px-3.5 py-2.5 text-sm text-danger-700" role="alert">
            {submitError}
          </div>
        )}

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Title" htmlFor="regulation-title" required error={errors.title}>
            <Input id="regulation-title" value={title} onChange={(e) => setTitle(e.target.value)} autoFocus />
          </Field>
          <Field label="Code" htmlFor="regulation-code" required error={errors.code} hint="Unique identifier, e.g. GDPR-5.">
            <Input id="regulation-code" value={code} onChange={(e) => setCode(e.target.value)} />
          </Field>
        </div>

        <Field label="Description" htmlFor="regulation-description">
          <Textarea id="regulation-description" rows={3} value={description} onChange={(e) => setDescription(e.target.value)} />
        </Field>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Jurisdiction" htmlFor="regulation-jurisdiction" hint="e.g. EU, US, global.">
            <Input id="regulation-jurisdiction" value={jurisdiction} onChange={(e) => setJurisdiction(e.target.value)} />
          </Field>
          <Field label="Status" htmlFor="regulation-status">
            <Select
              id="regulation-status"
              value={status}
              onChange={(e) => setStatus(e.target.value)}
              options={REGULATION_STATUSES.map((s) => ({ value: s, label: statusMeta[s]?.label ?? s }))}
            />
          </Field>
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Effective date" htmlFor="regulation-effective">
            <Input id="regulation-effective" type="date" value={effectiveDate} onChange={(e) => setEffectiveDate(e.target.value)} />
          </Field>
          <Field label="Expiry date" htmlFor="regulation-expiry" error={errors.expiry_date}>
            <Input id="regulation-expiry" type="date" value={expiryDate} onChange={(e) => setExpiryDate(e.target.value)} />
          </Field>
        </div>
      </form>
    </Modal>
  );
}
