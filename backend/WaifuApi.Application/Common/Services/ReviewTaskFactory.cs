using System;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Common.Services;

/// <summary>
/// Builds the <see cref="ReviewTask"/> that accompanies newly created content. Used by the upload/create
/// handlers so every submission has a durable, uniform review record (whether reviewed or auto-accepted).
/// </summary>
public static class ReviewTaskFactory
{
    public static ReviewTask NewContent(ReviewableContentType type, long targetId, long? submitterId, bool requiresReview)
    {
        var now = DateTime.UtcNow;
        return new ReviewTask
        {
            Kind = ReviewTaskKind.NewContent,
            TargetType = type,
            TargetId = targetId,
            SubmitterId = submitterId,
            Status = requiresReview ? ReviewTaskStatus.Pending : ReviewTaskStatus.Approved,
            CreatedAt = now,
            ResolvedAt = requiresReview ? null : now,
            ReviewerNote = requiresReview ? null : "Auto-accepted (trusted role)."
        };
    }
}
