export type TocItem = {
  id: string;
  label: string;
};

type TableOfContentsProps = {
  items: TocItem[];
};

export function TableOfContents({ items }: TableOfContentsProps) {
  return (
    <aside className="toc" aria-label="Table of contents">
      {items.map((item) => (
        <button
          key={item.id}
          type="button"
          onClick={() =>
            document.getElementById(item.id)?.scrollIntoView({
              behavior: window.matchMedia("(prefers-reduced-motion: reduce)")
                .matches
                ? "instant"
                : "smooth",
              block: "start",
            })
          }
        >
          {item.label}
        </button>
      ))}
    </aside>
  );
}
