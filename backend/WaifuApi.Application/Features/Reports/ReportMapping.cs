using WaifuApi.Application.Common.Extensions;
using WaifuApi.Application.Common.Models;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Reports;

/// <summary>Shared mapping from a <see cref="Report"/> entity to its DTO (reused by GetReports and UpdateReport).</summary>
public static class ReportMapping
{
    public static ReportDto ToDto(Report report, string cdnBaseUrl, bool includeUploaderId)
    {
        return new ReportDto
        {
            Id = report.Id,
            UserId = report.UserId,
            User = report.User != null
                ? new UserMinimalDto { Id = report.User.Id, Name = report.User.Name, AvatarUrl = report.User.AvatarUrl }
                : null,
            ImageId = report.ImageId,
            Image = report.Image != null ? report.Image.ToDto(cdnBaseUrl, includeUploaderId) : null,
            Description = report.Description,
            Status = report.Status,
            ReviewerNote = report.ReviewerNote,
            CreatedAt = report.CreatedAt,
        };
    }
}
