import { useEffect, useRef, useState } from 'react';
import { TermsDialog } from './TermsDialog';
import { setTermsHost, type TermsRequest } from '../lib/termsRequest';

/** Shows the terms dialog for the install and update flows (lib/termsRequest).
 *  One request at a time: a new one answers the open one with a cancel. */
export function TermsHost() {
  const [req, setReq] = useState<TermsRequest | null>(null);
  const current = useRef<TermsRequest | null>(null);

  useEffect(() => setTermsHost((next) => {
    current.current?.done(false);
    current.current = next;
    setReq(next);
  }), []);

  if (!req) return null;
  const finish = (accepted: boolean) => {
    const open = current.current;
    current.current = null;
    setReq(null);
    open?.done(accepted);
  };
  return <TermsDialog game={req.game} terms={req.terms} onAccepted={() => finish(true)} onCancel={() => finish(false)} />;
}
