using System;
using System.Text.Json.Nodes;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Common.Models;

public class ReviewTaskDto
{
    public long Id { get; set; }
    public ReviewTaskKind Kind { get; set; }

    public long? SubmitterId { get; set; }
    public UserMinimalDto? Submitter { get; set; }

    public ReviewableContentType TargetType { get; set; }
    public long TargetId { get; set; }

    /// <summary>The proposed changes (Edit tasks only; shape depends on <see cref="TargetType"/>).</summary>
    public JsonNode? Payload { get; set; }

    /// <summary>The current state of the target entity, or null if it no longer exists (e.g. a rejected submission).</summary>
    public ReviewTaskTargetDto? Target { get; set; }

    public string? Reason { get; set; }
    public ReviewTaskStatus Status { get; set; }

    public long? ReviewerId { get; set; }
    public UserMinimalDto? Reviewer { get; set; }
    public string? ReviewerNote { get; set; }

    /// <summary>Set when a moderator (other than the submitter) edited this submission's content.</summary>
    public DateTime? ModeratorEditedAt { get; set; }

    public DateTime CreatedAt { get; set; }
    public DateTime? ResolvedAt { get; set; }
}

/// <summary>
/// The current state of a review task's target. Exactly one property is populated,
/// matching <see cref="ReviewTaskDto.TargetType"/> (all null if the target was deleted).
/// </summary>
public class ReviewTaskTargetDto
{
    public ImageDto? Image { get; set; }
    public TagDto? Tag { get; set; }
    public ArtistDto? Artist { get; set; }
}
