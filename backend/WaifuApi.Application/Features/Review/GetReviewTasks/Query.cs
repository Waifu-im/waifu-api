using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using WaifuApi.Application.Common.Constants;
using WaifuApi.Application.Common.Extensions;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Common.Utilities;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Review.GetReviewTasks;

public class GetReviewTasksQuery : IQuery<PaginatedList<ReviewTaskDto>>
{
    public ReviewTaskStatus? Status { get; set; }
    public ReviewTaskKind? Kind { get; set; }
    public ReviewableContentType? TargetType { get; set; }
    public long? TargetId { get; set; }

    /// <summary>When set, only tasks submitted by this user are returned (scopes non-moderators to their own).</summary>
    public long? SubmitterId { get; set; }

    public int Page { get; set; } = 1;
    public int PageSize { get; set; }
}

public class GetReviewTasksQueryHandler : IQueryHandler<GetReviewTasksQuery, PaginatedList<ReviewTaskDto>>
{
    private readonly IWaifuDbContext _context;
    private readonly string _cdnBaseUrl;
    private readonly int _defaultPageSize;
    private readonly int _maxPageSize;

    public GetReviewTasksQueryHandler(IWaifuDbContext context, IConfiguration configuration)
    {
        _context = context;
        _cdnBaseUrl = configuration[ConfigurationKeys.Cdn.BaseUrl] ?? throw new InvalidOperationException("Cdn:BaseUrl is required.");
        _defaultPageSize = int.TryParse(configuration[ConfigurationKeys.Review.DefaultPageSize], out var d) ? d : 30;
        _maxPageSize = int.TryParse(configuration[ConfigurationKeys.Review.MaxPageSize], out var m) ? m : -1;
    }

    public async ValueTask<PaginatedList<ReviewTaskDto>> Handle(GetReviewTasksQuery request, CancellationToken cancellationToken)
    {
        var (page, pageSize) = PaginationUtils.Normalize(request.Page, request.PageSize, _defaultPageSize, _maxPageSize);

        var query = _context.ReviewTasks
            .AsNoTracking()
            .Include(t => t.Submitter)
            .Include(t => t.Reviewer)
            .AsQueryable();

        if (request.Status.HasValue) query = query.Where(t => t.Status == request.Status.Value);
        if (request.Kind.HasValue) query = query.Where(t => t.Kind == request.Kind.Value);
        if (request.TargetType.HasValue) query = query.Where(t => t.TargetType == request.TargetType.Value);
        if (request.TargetId.HasValue) query = query.Where(t => t.TargetId == request.TargetId.Value);
        if (request.SubmitterId.HasValue) query = query.Where(t => t.SubmitterId == request.SubmitterId.Value);

        query = query.OrderByDescending(t => t.CreatedAt);

        var pagedTasks = await PaginatedList<ReviewTask>.CreateAsync(query, page, pageSize, _maxPageSize, _defaultPageSize, cancellationToken);

        var targets = await LoadTargetsAsync(pagedTasks.Items, cancellationToken);
        var dtos = pagedTasks.Items.Select(t => ReviewTaskMapping.ToDto(t, targets.Build(t))).ToList();

        return new PaginatedList<ReviewTaskDto>(dtos, pagedTasks.TotalCount, pagedTasks.PageNumber, pageSize, _maxPageSize, _defaultPageSize);
    }

    private async Task<TargetLookup> LoadTargetsAsync(IReadOnlyCollection<ReviewTask> tasks, CancellationToken ct)
    {
        var imageIds = tasks.Where(t => t.TargetType == ReviewableContentType.Image).Select(t => t.TargetId).Distinct().ToList();
        var tagIds = tasks.Where(t => t.TargetType == ReviewableContentType.Tag).Select(t => t.TargetId).Distinct().ToList();
        var artistIds = tasks.Where(t => t.TargetType == ReviewableContentType.Artist).Select(t => t.TargetId).Distinct().ToList();

        var images = imageIds.Count == 0
            ? new Dictionary<long, Image>()
            : await _context.Images.AsNoTracking()
                .Include(i => i.Tags)
                .Include(i => i.Artists)
                .Where(i => imageIds.Contains(i.Id))
                .ToDictionaryAsync(i => i.Id, ct);

        var tags = tagIds.Count == 0
            ? new Dictionary<long, Tag>()
            : await _context.Tags.AsNoTracking().Where(t => tagIds.Contains(t.Id)).ToDictionaryAsync(t => t.Id, ct);

        var artists = artistIds.Count == 0
            ? new Dictionary<long, Artist>()
            : await _context.Artists.AsNoTracking().Where(a => artistIds.Contains(a.Id)).ToDictionaryAsync(a => a.Id, ct);

        return new TargetLookup(images, tags, artists, _cdnBaseUrl);
    }

    private sealed class TargetLookup
    {
        private readonly Dictionary<long, Image> _images;
        private readonly Dictionary<long, Tag> _tags;
        private readonly Dictionary<long, Artist> _artists;
        private readonly string _cdnBaseUrl;

        public TargetLookup(Dictionary<long, Image> images, Dictionary<long, Tag> tags, Dictionary<long, Artist> artists, string cdnBaseUrl)
        {
            _images = images;
            _tags = tags;
            _artists = artists;
            _cdnBaseUrl = cdnBaseUrl;
        }

        public ReviewTaskTargetDto? Build(ReviewTask task)
        {
            switch (task.TargetType)
            {
                case ReviewableContentType.Image:
                    return _images.TryGetValue(task.TargetId, out var image)
                        ? new ReviewTaskTargetDto { Image = image.ToDto(_cdnBaseUrl, includeUploaderId: true) }
                        : null;
                case ReviewableContentType.Tag:
                    return _tags.TryGetValue(task.TargetId, out var tag)
                        ? new ReviewTaskTargetDto { Tag = tag.ToDto() }
                        : null;
                case ReviewableContentType.Artist:
                    return _artists.TryGetValue(task.TargetId, out var artist)
                        ? new ReviewTaskTargetDto { Artist = artist.ToDto() }
                        : null;
                default:
                    return null;
            }
        }
    }
}
