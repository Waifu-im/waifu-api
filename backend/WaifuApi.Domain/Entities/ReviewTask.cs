using System;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Domain.Entities;

/// <summary>
/// A unit of review work: either a newly submitted image/tag/artist awaiting approval
/// (<see cref="ReviewTaskKind.NewContent"/>) or a proposed change to existing content
/// (<see cref="ReviewTaskKind.Edit"/>).
///
/// The task is the durable source of truth for the *review workflow* (who submitted, who resolved,
/// when, why) and is what powers the review queue and the user's "my submissions" view. The
/// content's public visibility remains the entity's own <c>ReviewStatus</c>. The association to the
/// target is polymorphic, so there is no DB foreign key to the target — and the task deliberately
/// survives the target being deleted (e.g. a rejected upload), preserving the audit trail.
/// </summary>
public class ReviewTask
{
    public long Id { get; set; }

    public ReviewTaskKind Kind { get; set; }

    public ReviewableContentType TargetType { get; set; }

    /// <summary>The id of the targeted Image/Tag/Artist. May point at an entity that no longer exists (rejected submission).</summary>
    public long TargetId { get; set; }

    /// <summary>The user who submitted the upload or proposed the edit. Nullable so history survives user deletion.</summary>
    public long? SubmitterId { get; set; }
    public User? Submitter { get; set; }

    /// <summary>JSON (jsonb) proposed field-level changes. Only used by <see cref="ReviewTaskKind.Edit"/>.</summary>
    public string? Payload { get; set; }

    /// <summary>Optional free-text justification provided by the submitter.</summary>
    public string? Reason { get; set; }

    public ReviewTaskStatus Status { get; set; } = ReviewTaskStatus.Pending;

    /// <summary>The moderator/admin who resolved the task (null while pending or when auto-resolved).</summary>
    public long? ReviewerId { get; set; }
    public User? Reviewer { get; set; }

    /// <summary>Moderator note shown to the submitter (e.g. the rejection reason).</summary>
    public string? ReviewerNote { get; set; }

    /// <summary>Set when a moderator (other than the submitter) edited this submission's content — used to show the submitter a "modified by a moderator" cue.</summary>
    public DateTime? ModeratorEditedAt { get; set; }

    public DateTime CreatedAt { get; set; }
    public DateTime? ResolvedAt { get; set; }
}
