package svc

import (
	"fmt"

	"appforge/common/secretbox"
	"appforge/services/core/internal/agentpki"
	"appforge/services/core/internal/config"
	"appforge/services/core/models"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config                  config.Config
	DB                      sqlx.SqlConn
	ApplicationModel        models.TAppApplicationModel
	VersionModel            models.TAppVersionModel
	SigningConfigModel      models.TAppSigningConfigModel
	ChannelModel            models.TPromotionChannelModel
	BuildTaskModel          models.TBuildTaskModel
	InstallModel            models.TChannelInstallModel
	ChannelEventModel       models.TChannelEventModel
	StorageObjectModel      models.TStorageObjectModel
	BrandingProfileModel    models.TAppBrandingProfileModel
	BrandingPreflightModel  models.TBrandingPreflightModel
	WhiteLabelTemplateModel models.TWhiteLabelTemplateModel
	WhiteLabelRevisionModel models.TWhiteLabelTemplateRevisionModel
	WhiteLabelProductModel  models.TWhiteLabelProductModel
	PackageCertificateModel models.TPackageCertificateBindingModel
	BuilderNodeModel        models.TBuilderNodeModel
	BuildPolicyModel        models.TBuildConcurrencyPolicyModel
	BuildFairQueueModel     models.TBuildFairQueueModel
	BuildSlotLeaseModel     models.TBuildSlotLeaseModel
	BuildCacheModel         models.TBuildCacheEntryModel
	BuildSchedulerModel     models.TBuildSchedulerEventModel
	OpenApiCredentialModel  models.TOpenApiCredentialModel
	OpenApiIdempotencyModel models.TOpenApiIdempotencyModel
	OpenApiAuditModel       models.TOpenApiAuditModel
	WebhookEndpointModel    models.TWebhookEndpointModel
	WebhookDeliveryModel    models.TWebhookDeliveryModel
	OutboxEventModel        models.TOutboxEventModel
	SourceIntegrationModel  models.TSourceIntegrationModel
	SourceRepositoryModel   models.TSourceRepositoryModel
	SourceArtifactModel     models.TSourceArtifactModel
	SourceBuildTriggerModel models.TSourceBuildTriggerModel
	SourceWebhookEventModel models.TSourceWebhookEventModel
	BillingPlanModel        models.TBillingPlanModel
	TenantSubscriptionModel models.TTenantSubscriptionModel
	TenantEntitlementModel  models.TTenantEntitlementModel
	UsageLedgerModel        models.TUsageLedgerModel
	QuotaReservationModel   models.TQuotaReservationModel
	UsageThresholdModel     models.TUsageThresholdNotificationModel
	InvoiceModel            models.TInvoiceModel
	InvoiceItemModel        models.TInvoiceItemModel
	PaymentTransactionModel models.TPaymentTransactionModel
	BillingWebhookModel     models.TBillingWebhookEventModel
	LocalAgentModel         models.TLocalAgentModel
	LocalAgentRegistration  models.TLocalAgentRegistrationModel
	LocalAgentCertificate   models.TLocalAgentCertificateModel
	LocalAgentCapability    models.TLocalAgentCapabilityModel
	HybridArtifactModel     models.THybridArtifactReferenceModel
	AirGappedPackageModel   models.TAirGappedPackageModel
	AgentPKI                *agentpki.Signer
	Secrets                 *secretbox.Box
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Mysql.DataSource)
	secrets, err := secretbox.New(c.SigningSecrets.MasterKeyBase64)
	if err != nil {
		panic(fmt.Sprintf("initialize core secret encryption: %v", err))
	}
	agentSigner, err := agentpki.New(c.AgentPKI.CACertificateFile, c.AgentPKI.CAPrivateKeyFile, c.AgentPKI.CertificateTTL)
	if err != nil {
		panic(fmt.Sprintf("initialize Local Agent PKI: %v", err))
	}
	return &ServiceContext{
		Config:                  c,
		DB:                      conn,
		ApplicationModel:        models.NewTAppApplicationModel(conn, c.CacheRedis),
		VersionModel:            models.NewTAppVersionModel(conn, c.CacheRedis),
		SigningConfigModel:      models.NewTAppSigningConfigModel(conn, c.CacheRedis),
		ChannelModel:            models.NewTPromotionChannelModel(conn, c.CacheRedis),
		BuildTaskModel:          models.NewTBuildTaskModel(conn, c.CacheRedis),
		InstallModel:            models.NewTChannelInstallModel(conn, c.CacheRedis),
		ChannelEventModel:       models.NewTChannelEventModel(conn, c.CacheRedis),
		StorageObjectModel:      models.NewTStorageObjectModel(conn, c.CacheRedis),
		BrandingProfileModel:    models.NewTAppBrandingProfileModel(conn, c.CacheRedis),
		BrandingPreflightModel:  models.NewTBrandingPreflightModel(conn, c.CacheRedis),
		WhiteLabelTemplateModel: models.NewTWhiteLabelTemplateModel(conn, c.CacheRedis),
		WhiteLabelRevisionModel: models.NewTWhiteLabelTemplateRevisionModel(conn, c.CacheRedis),
		WhiteLabelProductModel:  models.NewTWhiteLabelProductModel(conn, c.CacheRedis),
		PackageCertificateModel: models.NewTPackageCertificateBindingModel(conn, c.CacheRedis),
		BuilderNodeModel:        models.NewTBuilderNodeModel(conn, c.CacheRedis),
		BuildPolicyModel:        models.NewTBuildConcurrencyPolicyModel(conn, c.CacheRedis),
		BuildFairQueueModel:     models.NewTBuildFairQueueModel(conn, c.CacheRedis),
		BuildSlotLeaseModel:     models.NewTBuildSlotLeaseModel(conn, c.CacheRedis),
		BuildCacheModel:         models.NewTBuildCacheEntryModel(conn, c.CacheRedis),
		BuildSchedulerModel:     models.NewTBuildSchedulerEventModel(conn, c.CacheRedis),
		OpenApiCredentialModel:  models.NewTOpenApiCredentialModel(conn, c.CacheRedis),
		OpenApiIdempotencyModel: models.NewTOpenApiIdempotencyModel(conn, c.CacheRedis),
		OpenApiAuditModel:       models.NewTOpenApiAuditModel(conn, c.CacheRedis),
		WebhookEndpointModel:    models.NewTWebhookEndpointModel(conn, c.CacheRedis),
		WebhookDeliveryModel:    models.NewTWebhookDeliveryModel(conn, c.CacheRedis),
		OutboxEventModel:        models.NewTOutboxEventModel(conn, c.CacheRedis),
		SourceIntegrationModel:  models.NewTSourceIntegrationModel(conn, c.CacheRedis),
		SourceRepositoryModel:   models.NewTSourceRepositoryModel(conn, c.CacheRedis),
		SourceArtifactModel:     models.NewTSourceArtifactModel(conn, c.CacheRedis),
		SourceBuildTriggerModel: models.NewTSourceBuildTriggerModel(conn, c.CacheRedis),
		SourceWebhookEventModel: models.NewTSourceWebhookEventModel(conn, c.CacheRedis),
		BillingPlanModel:        models.NewTBillingPlanModel(conn, c.CacheRedis),
		TenantSubscriptionModel: models.NewTTenantSubscriptionModel(conn, c.CacheRedis),
		TenantEntitlementModel:  models.NewTTenantEntitlementModel(conn, c.CacheRedis),
		UsageLedgerModel:        models.NewTUsageLedgerModel(conn, c.CacheRedis),
		QuotaReservationModel:   models.NewTQuotaReservationModel(conn, c.CacheRedis),
		UsageThresholdModel:     models.NewTUsageThresholdNotificationModel(conn, c.CacheRedis),
		InvoiceModel:            models.NewTInvoiceModel(conn, c.CacheRedis),
		InvoiceItemModel:        models.NewTInvoiceItemModel(conn, c.CacheRedis),
		PaymentTransactionModel: models.NewTPaymentTransactionModel(conn, c.CacheRedis),
		BillingWebhookModel:     models.NewTBillingWebhookEventModel(conn, c.CacheRedis),
		LocalAgentModel:         models.NewTLocalAgentModel(conn, c.CacheRedis),
		LocalAgentRegistration:  models.NewTLocalAgentRegistrationModel(conn, c.CacheRedis),
		LocalAgentCertificate:   models.NewTLocalAgentCertificateModel(conn, c.CacheRedis),
		LocalAgentCapability:    models.NewTLocalAgentCapabilityModel(conn, c.CacheRedis),
		HybridArtifactModel:     models.NewTHybridArtifactReferenceModel(conn, c.CacheRedis),
		AirGappedPackageModel:   models.NewTAirGappedPackageModel(conn, c.CacheRedis),
		AgentPKI:                agentSigner,
		Secrets:                 secrets,
	}
}
