using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Interfaces;

public interface ITokenService
{
    string GenerateJwtToken(User user);
}
