export default function MachineStatusBadge({ enabled }: { enabled: boolean }) {
  return <span className={`inline-flex shrink-0 items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium ${enabled ? 'bg-success-soft text-success' : 'bg-danger-soft text-danger'}`}>
    <span className={`size-1.5 shrink-0 rounded-full ${enabled ? 'bg-success' : 'bg-danger'}`} aria-hidden />
    {enabled ? '启用' : '已停用'}
  </span>
}
