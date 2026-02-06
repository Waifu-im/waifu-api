using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Interfaces;

namespace WaifuApi.Application.Features.Stats.GetPublicStats;

public record GetPublicStatsQuery : IRequest<PublicStatsDto>;

public class GetPublicStatsQueryHandler : IRequestHandler<GetPublicStatsQuery, PublicStatsDto>
{
    private readonly IWaifuDbContext _context;

    public GetPublicStatsQueryHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<PublicStatsDto> Handle(GetPublicStatsQuery request, CancellationToken cancellationToken)
    {
        // Get total requests from GlobalStats table
        var globalStat = await _context.GlobalStats.FirstOrDefaultAsync(s => s.Key == "TotalRequests", cancellationToken);
        var totalRequests = globalStat?.Value ?? 0;
        
        var totalImages = await _context.Images.CountAsync(cancellationToken);
        var totalTags = await _context.Tags.CountAsync(cancellationToken);
        var totalArtists = await _context.Artists.CountAsync(cancellationToken);

        return new PublicStatsDto
        {
            TotalRequests = totalRequests,
            TotalImages = totalImages,
            TotalTags = totalTags,
            TotalArtists = totalArtists
        };
    }
}

public class PublicStatsDto
{
    public long TotalRequests { get; set; }
    public int TotalImages { get; set; }
    public int TotalTags { get; set; }
    public int TotalArtists { get; set; }
}