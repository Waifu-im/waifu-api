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
using WaifuApi.Application.Interfaces;

namespace WaifuApi.Application.Features.Images.GetStorageDiff;

public enum StorageDiffMode
{
    OrphansInS3,
    MissingFromS3
}

public class StorageDiffDto
{
    public long? Id { get; set; }
    public string FileName { get; set; } = string.Empty;
    public ImageDto? Image { get; set; }
}

public class GetStorageDiffQuery : IQuery<PaginatedList<StorageDiffDto>>
{
    public StorageDiffMode Mode { get; set; }
    public int Page { get; set; } = 1;
    public int PageSize { get; set; }
}

public class GetStorageDiffQueryHandler : IQueryHandler<GetStorageDiffQuery, PaginatedList<StorageDiffDto>>
{
    private readonly IWaifuDbContext _context;
    private readonly IStorageService _storageService;
    private readonly string _cdnBaseUrl;
    private readonly int _defaultPageSize;
    private readonly int _maxPageSize;

    public GetStorageDiffQueryHandler(IWaifuDbContext context, IStorageService storageService, IConfiguration configuration)
    {
        _context = context;
        _storageService = storageService;
        _cdnBaseUrl = configuration[ConfigurationKeys.Cdn.BaseUrl] ?? throw new InvalidOperationException($"{ConfigurationKeys.Cdn.BaseUrl} is required.");
        _defaultPageSize = int.Parse(configuration[ConfigurationKeys.StorageDiff.DefaultPageSize] ?? throw new InvalidOperationException($"{ConfigurationKeys.StorageDiff.DefaultPageSize} is required."));
        _maxPageSize = int.Parse(configuration[ConfigurationKeys.StorageDiff.MaxPageSize] ?? throw new InvalidOperationException($"{ConfigurationKeys.StorageDiff.MaxPageSize} is required."));
    }

    public async ValueTask<PaginatedList<StorageDiffDto>> Handle(GetStorageDiffQuery request, CancellationToken cancellationToken)
    {
        var pageSize = request.PageSize == 0 ? _defaultPageSize : request.PageSize;
        if (_maxPageSize > 0 && pageSize > _maxPageSize) pageSize = _maxPageSize;
        var page = request.Page < 1 ? 1 : request.Page;

        var allS3Keys = await _storageService.ListObjectsAsync(cancellationToken);
        var s3Keys = allS3Keys.Where(k => !k.StartsWith("healthcheck/")).ToList();
        var s3Set = new HashSet<string>(s3Keys);

        var dbImages = await _context.Images.AsNoTracking()
            .Include(i => i.Tags)
            .Include(i => i.Artists)
            .ToListAsync(cancellationToken);

        var dbFileMap = dbImages.ToDictionary(i => $"{i.Id}{i.Extension}", i => i);

        List<StorageDiffDto> results;

        if (request.Mode == StorageDiffMode.OrphansInS3)
        {
            var dbFileNames = new HashSet<string>(dbFileMap.Keys);
            results = s3Keys
                .Where(key => !dbFileNames.Contains(key))
                .Select(key => new StorageDiffDto
                {
                    FileName = key
                })
                .ToList();
        }
        else
        {
            results = dbImages
                .Where(i => !s3Set.Contains($"{i.Id}{i.Extension}"))
                .Select(i => new StorageDiffDto
                {
                    Id = i.Id,
                    FileName = $"{i.Id}{i.Extension}",
                    Image = i.ToDto(_cdnBaseUrl, true)
                })
                .ToList();
        }

        var totalCount = results.Count;
        var paged = results.Skip((page - 1) * pageSize).Take(pageSize).ToList();
        return new PaginatedList<StorageDiffDto>(paged, totalCount, page, pageSize, _maxPageSize, _defaultPageSize);
    }
}
