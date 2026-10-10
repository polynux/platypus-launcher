import type { AnchorHTMLAttributes } from 'react';
import { openExternal, webURL } from '../lib/external';

/** A link to a web page. Clicks open it in the user's browser through
 *  openExternal; an address that is not http(s) renders with no href. */
export function ExternalLink({ href, onClick, ...rest }: AnchorHTMLAttributes<HTMLAnchorElement>) {
  const url = webURL(href);
  return (
    <a
      {...rest}
      href={url || undefined}
      onClick={(e) => {
        onClick?.(e);
        if (!e.defaultPrevented && url) {
          e.preventDefault();
          openExternal(url);
        }
      }}
    />
  );
}
