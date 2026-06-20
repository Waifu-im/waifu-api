import { Info } from 'lucide-react';
import { ReviewStatus } from '../types';

/**
 * Heads-up shown in the moderator edit modals: flipping a currently-pending item's status to Accepted also
 * auto-approves its pending submission in the review queue (the entity and its NewContent task are kept in
 * sync by the update commands). Only the NewContent submission is touched; separate edit suggestions are not.
 */
export const AcceptSubmissionNote = ({ current, selected }: { current?: ReviewStatus; selected?: ReviewStatus }) => {
    if (current !== ReviewStatus.Pending || selected !== ReviewStatus.Accepted) return null;
    return (
        <div className="mt-2 flex items-start gap-2 rounded-lg border border-amber-500/20 bg-amber-500/10 p-2.5 text-xs text-amber-600">
            <Info size={14} className="mt-0.5 shrink-0" />
            <span>Accepting this also approves any pending submission for it in the review queue. Edit suggestions aren't affected.</span>
        </div>
    );
};

export default AcceptSubmissionNote;
