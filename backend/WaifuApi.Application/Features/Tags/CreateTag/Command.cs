using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using WaifuApi.Application.Common.Exceptions;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Tags.CreateTag;

public record CreateTagCommand(string Name, string Description) : ICommand<Tag>;

public class CreateTagCommandHandler : ICommandHandler<CreateTagCommand, Tag>
{
    private readonly IWaifuDbContext _context;
    private readonly IConfiguration _configuration;

    public CreateTagCommandHandler(IWaifuDbContext context, IConfiguration configuration)
    {
        _context = context;
        _configuration = configuration;
    }

    public async ValueTask<Tag> Handle(CreateTagCommand request, CancellationToken cancellationToken)
    {
        var exists = await _context.Tags.AnyAsync(t => t.Name == request.Name, cancellationToken);
        if (exists)
        {
            throw new ConflictException($"Tag with name '{request.Name}' already exists.");
        }

        var requireReview = bool.Parse(_configuration["Moderation:RequireTagReview"] ?? "true");
        
        var tag = new Tag
        {
            Name = request.Name,
            Description = request.Description,
            ReviewStatus = requireReview ? ReviewStatus.Pending : ReviewStatus.Accepted
        };

        _context.Tags.Add(tag);
        await _context.SaveChangesAsync(cancellationToken);

        return tag;
    }
}
