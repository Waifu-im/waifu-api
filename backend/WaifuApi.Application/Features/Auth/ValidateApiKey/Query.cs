using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Auth.ValidateApiKey;

public record ValidateApiKeyQuery(string KeyString) : IQuery<ApiKey?>;

public class ValidateApiKeyQueryHandler : IQueryHandler<ValidateApiKeyQuery, ApiKey?>
{
    private readonly IWaifuDbContext _context;

    public ValidateApiKeyQueryHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<ApiKey?> Handle(ValidateApiKeyQuery request, CancellationToken cancellationToken)
    {
        var key = await _context.ApiKeys
            .Include(k => k.User)
            .FirstOrDefaultAsync(k => k.Key == request.KeyString, cancellationToken);
            
        return key;
    }
}
