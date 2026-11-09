export function BrandMark({ size = 29 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      className="brand-logo"
      viewBox="0 0 32 32"
      fill="none"
      aria-hidden="true"
    >
      <path
        d="M11 4H4v24h7M21 4h7v24h-7M16 14v9"
        stroke="currentColor"
        strokeWidth="2.2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <circle cx="16" cy="9" r="1.5" fill="currentColor" />
    </svg>
  );
}
