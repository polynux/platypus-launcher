// Layouts at runtime (issue #96, #109). The core validates a theme's layouts
// XML and sends typed regions over RPC; this maps the names onto the view's
// block components. Regions and rows are plain divs: spacing and columns live
// in the view's region and row frames, so the built-in layout reproduces
// today's look exactly, and a block that renders null leaves no gap behind.
// A row with span attributes shares its 12-column grid by span instead of
// splitting equally, so a tile grid beside a list gets the width it needs.
import type { ReactNode } from 'react';
import type { ThemeLayout, ThemeLayoutNode as LayoutNode } from '../types';

/** One row's frame. A string applies to every row; a function picks the
 *  frame per row (its blocks) and region, returning undefined for the
 *  default equal-columns grid. A returned frame replaces the default
 *  entirely, columns included, so a view can give a row its own ratio. */
export type RowClassName = string | ((row: LayoutNode[], region: string) => string | undefined);

interface ThemeLayoutProps {
  layout: ThemeLayout;
  /** The view's blocks by layout name. Names the core validated always
   *  resolve; a missing one renders nothing, never an error. Each block
   *  renders a single root, so a row counts one column per block. */
  blocks: Record<string, ReactNode>;
  /** The regions wrapper. */
  className?: string;
  /** One region's frame, by region name. */
  regionClassName?: (region: string) => string | undefined;
  /** One row's frame; the default column count comes from the row's block count. */
  rowClassName?: RowClassName;
}

export function ThemeLayout({ layout, blocks, className, regionClassName, rowClassName }: ThemeLayoutProps) {
  return (
    <div className={className}>
      {layout.regions.map((region) => (
        <div key={region.name} data-region={region.name} className={regionClassName?.(region.name)}>
          {region.nodes.map((node, index) => (
            <ThemeLayoutNode key={index} node={node} blocks={blocks} rowClassName={rowClassName} region={region.name} />
          ))}
        </div>
      ))}
    </div>
  );
}

function ThemeLayoutNode({ node, blocks, rowClassName, region }: { node: LayoutNode; blocks: Record<string, ReactNode>; rowClassName?: RowClassName; region: string }) {
  if (node.row) {
    // A row the core validated always groups blocks; an empty one would
    // paint an empty grid, so it renders nothing.
    if (node.row.length === 0) return null;
    // A row share (issue #109 span): a tile grid beside a list needs more
    // columns than an equal split gives (Continue + Waiting). The core
    // validates span 1-12; the default is 1, so rows without span keep the
    // old equal-columns path pixel-identically.
    const spans = node.row.map((child) => {
      const raw = child.attrs?.span;
      const parsed = raw === undefined ? NaN : Number.parseInt(raw, 10);
      return Number.isFinite(parsed) && parsed >= 1 && parsed <= 12 ? parsed : 1;
    });
    if (spans.some((s) => s > 1)) {
      const total = spans.reduce((a, b) => a + b, 0);
      return (
        <div className="grid min-w-0 gap-5" style={{ gridTemplateColumns: `repeat(${total}, minmax(0, 1fr))` }}>
          {node.row.map((child, index) => (
            <div key={index} className="min-w-0" style={{ gridColumn: `span ${spans[index]} / span ${spans[index]}` }}>
              <ThemeLayoutNode node={child} blocks={blocks} rowClassName={rowClassName} region={region} />
            </div>
          ))}
        </div>
      );
    }
    const frame = typeof rowClassName === 'function' ? rowClassName(node.row, region) : rowClassName;
    if (frame !== undefined) {
      return (
        <div className={frame}>
          {node.row.map((child, index) => (
            <ThemeLayoutNode key={index} node={child} blocks={blocks} rowClassName={rowClassName} region={region} />
          ))}
        </div>
      );
    }
    return (
      <div className="grid gap-5" style={{ gridTemplateColumns: `repeat(${node.row.length}, minmax(0, 1fr))` }}>
        {node.row.map((child, index) => (
          <ThemeLayoutNode key={index} node={child} blocks={blocks} rowClassName={rowClassName} region={region} />
        ))}
      </div>
    );
  }
  if (!node.block) return null;
  return <>{blocks[node.block] ?? null}</>;
}
