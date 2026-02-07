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

    public DeleteS3ObjectsCommandHandler(IStorageService storageService)
    {
        _storageService = storageService;
    }

    public async ValueTask<Unit> Handle(DeleteS3ObjectsCommand request, CancellationToken cancellationToken)
    {
        foreach (var fileName in request.FileNames)
        {
            await _storageService.DeleteAsync(fileName);
        }

        return Unit.Value;
    }
}
