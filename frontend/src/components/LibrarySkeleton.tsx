import { MonoLabel } from './ui';

/** Placeholder tiles for the first library scan: the library is pending, so it
 *  shows no count and no empty state until the scan answers. */
export function LibrarySkeleton() {
  return (
    <div className="p-7" aria-busy="true">
      <MonoLabel className="mb-4">scanning library…</MonoLabel>
      <div className="grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-4">
        {Array.from({ length: 8 }, (_, i) => (
          <div key={i} className="aspect-[3/4] animate-pulse rounded-2xl bg-white/[0.045] ring-1 ring-white/[0.07] motion-reduce:animate-none" />
        ))}
      </div>
    </div>
  );
}
