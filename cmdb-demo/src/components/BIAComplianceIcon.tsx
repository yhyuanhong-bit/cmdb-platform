export default function BIAComplianceIcon({ ok }: { ok: boolean }) {
  return (
    <span
      className={`material-symbols-outlined text-lg ${
        ok ? 'text-[#34d399]' : 'text-on-surface-variant/40'
      }`}
    >
      {ok ? 'check_circle' : 'cancel'}
    </span>
  )
}
