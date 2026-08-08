import { useEffect, useState, type FormEvent } from "react";
import { Button } from "@/components/ui/Button";
import { Field } from "@/components/ui/Field";
import { Input } from "@/components/ui/Input";
import { Modal } from "@/components/ui/Modal";
import { SearchableSelect } from "@/components/ui/SearchableSelect";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { ApiError } from "@/types/api";
import { COMPLIANCE_STATUSES, SEVERITIES, type Compliance } from "@/types/entities";
import { riskMeta, statusMeta } from "@/lib/constants";

export interface ComplianceFormValues {
  name: string;
  description: string;
  status: string;
  risk_level: string;
  owner_id: string;
  regulation_id: string;
  due_date: string | null;
}

export interface ComplianceFormModalProps {
  open: boolean;
  onClose: () => void;
  compliance: Compliance | null;
  onSubmit: (values: ComplianceFormValues) => Promise<void>;
  submitting: boolean;
}

export function ComplianceFormModal({ open, onClose, compliance, onSubmit, submitting }: ComplianceFormModalProps) {
  const isEdit = Boolean(compliance);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [status, setStatus] = useState("draft");
  const [riskLevel, setRiskLevel] = useState("medium");
  const [ownerId, setOwnerId] = useState("");
  const [regulationId, setRegulationId] = useState("");
  const [dueDate, setDueDate] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [submitError, setSubmitError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setName(compliance?.name ?? "");
    setDescription(compliance?.description ?? "");
    setStatus(compliance?.status ?? "draft");
    setRiskLevel(compliance?.risk_level ?? "medium");
    setOwnerId(compliance?.owner_id ?? "");
    setRegulationId(compliance?.regulation_id ?? "");
    setDueDate(compliance?.due_date ? compliance.due_date.slice(0, 10) : "");
    setErrors({});
    setSubmitError(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, compliance?.id]);

  const submit = async (ev: FormEvent) => {
    ev.preventDefault();
    const e: Record<string, string> = {};
    if (!name.trim()) e.name = "Name is required.";
    setErrors(e);
    if (Object.keys(e).length > 0) return;
    setSubmitError(null);
    try {
      await onSubmit({
        name: name.trim(),
        description,
        status,
        risk_level: riskLevel,
        owner_id: ownerId.trim(),
        regulation_id: regulationId.trim(),
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
      title={isEdit ? "Edit compliance" : "New compliance"}
      description="A monitored compliance requirement with a status and risk level."
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={submitting}>
            Cancel
          </Button>
          <Button type="submit" form="compliance-form" loading={submitting}>
            {isEdit ? "Save changes" : "Create compliance"}
          </Button>
        </>
      }
    >
      <form id="compliance-form" onSubmit={submit} className="space-y-4" noValidate>
        {submitError && (
          <div className="rounded-lg border border-danger-200 bg-danger-50 px-3.5 py-2.5 text-sm text-danger-700" role="alert">
            {submitError}
          </div>
        )}

        <Field label="Name" htmlFor="compliance-name" required error={errors.name}>
          <Input id="compliance-name" value={name} onChange={(e) => setName(e.target.value)} autoFocus />
        </Field>

        <Field label="Description" htmlFor="compliance-description">
          <Textarea id="compliance-description" rows={3} value={description} onChange={(e) => setDescription(e.target.value)} />
        </Field>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Status" htmlFor="compliance-status">
            <Select
              id="compliance-status"
              value={status}
              onChange={(e) => setStatus(e.target.value)}
              options={COMPLIANCE_STATUSES.map((s) => ({ value: s, label: statusMeta[s]?.label ?? s }))}
            />
          </Field>
          <Field label="Risk level" htmlFor="compliance-risk">
            <Select
              id="compliance-risk"
              value={riskLevel}
              onChange={(e) => setRiskLevel(e.target.value)}
              options={SEVERITIES.map((s) => ({ value: s, label: riskMeta[s]?.label ?? s }))}
            />
          </Field>
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Owner" htmlFor="compliance-owner" hint="Optional operator account.">
            <SearchableSelect id="compliance-owner" entity="users" value={ownerId} onChange={setOwnerId} placeholder="Search user…" />
          </Field>
          <Field label="Due date" htmlFor="compliance-due">
            <Input id="compliance-due" type="date" value={dueDate} onChange={(e) => setDueDate(e.target.value)} />
          </Field>
        </div>

        <Field label="Regulation" htmlFor="compliance-regulation" hint="Optional governing regulation.">
          <SearchableSelect id="compliance-regulation" entity="regulations" value={regulationId} onChange={setRegulationId} placeholder="Search regulation…" />
        </Field>
      </form>
    </Modal>
  );
}
