using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using WaifuApi.Application.Interfaces;

namespace WaifuApi.Application.Features.Images.DeleteS3Objects;

public record DeleteS3ObjectsCommand(List<string> FileNames) : ICommand;

public class DeleteS3ObjectsCommandHandler : ICommandHandler<DeleteS3ObjectsCommand>
{
    private readonly IStorageService _storageService;
    private readonly ICdnCacheService _cdnCache;

    public DeleteS3ObjectsCommandHandler(IStorageService storageService, ICdnCacheService cdnCache)
    {
        _storageService = storageService;
        _cdnCache = cdnCache;
    }

    public async ValueTask<Unit> Handle(DeleteS3ObjectsCommand request, CancellationToken cancellationToken)
    {
        foreach (var fileName in request.FileNames)
        {
            await _storageService.DeleteAsync(fileName);
        }

        // Clear the CDN cache for just the deleted files so the edge stops serving them.
        await _cdnCache.PurgeFilesAsync(request.FileNames, cancellationToken);

        return Unit.Value;
    }
}
