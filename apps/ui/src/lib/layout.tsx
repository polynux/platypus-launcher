// Layouts at runtime (issue #96). The core validates a theme's layouts XML
// and sends typed regions over RPC; this maps the names onto the view's block
// components. Regions and rows are plain divs: spacing and columns live in
// the view's region and row frames, so the built-in layout reproduces
// today's look exactly, and a block that renders null leaves no gap behind.
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
