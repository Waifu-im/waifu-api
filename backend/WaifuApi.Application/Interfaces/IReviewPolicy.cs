using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Interfaces;

/// <summary>
/// Single source of truth for "does this action need moderator review?".
/// Reused by both the content-creation flow (uploads / new tags / new artists) and the
/// edit-request flow, so the rule for who is trusted enough to skip review lives in one place.
/// </summary>
public interface IReviewPolicy
{
    /// <summary>
    /// True if brand-new content of the given type, created by a user with the given role,
    /// must start as Pending and await moderator approval. False => auto-accepted.
    /// </summary>
    bool RequiresReviewForNewContent(ReviewableContentType type, Role? userRole);

    /// <summary>
    /// True if an edit request targeting the given content type, submitted by a user with the
    /// given role, must be queued for moderator approval. False => the change may be applied immediately.
    /// </summary>
    bool RequiresReviewForEditRequest(ReviewableContentType type, Role? userRole);
}
