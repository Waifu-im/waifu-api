using System.Text.Json.Nodes;
using WaifuApi.Application.Common.Models;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Review;

/// <summary>Shared mapping from the <see cref="ReviewTask"/> entity to its DTO.</summary>
public static class ReviewTaskMapping
{
    public static ReviewTaskDto ToDto(ReviewTask task, ReviewTaskTargetDto? target)
    {
        return new ReviewTaskDto
        {
            Id = task.Id,
            Kind = task.Kind,
            SubmitterId = task.SubmitterId,
            Submitter = task.Submitter != null
                ? new UserMinimalDto { Id = task.Submitter.Id, Name = task.Submitter.Name, AvatarUrl = task.Submitter.AvatarUrl }
                : null,
            TargetType = task.TargetType,
            TargetId = task.TargetId,
            Payload = task.Payload != null ? JsonNode.Parse(task.Payload) : null,
            Target = target,
            Reason = task.Reason,
            Status = task.Status,
            ReviewerId = task.ReviewerId,
            Reviewer = task.Reviewer != null
                ? new UserMinimalDto { Id = task.Reviewer.Id, Name = task.Reviewer.Name, AvatarUrl = task.Reviewer.AvatarUrl }
                : null,
            ReviewerNote = task.ReviewerNote,
            ModeratorEditedAt = task.ModeratorEditedAt,
            CreatedAt = task.CreatedAt,
            ResolvedAt = task.ResolvedAt
        };
    }
}
